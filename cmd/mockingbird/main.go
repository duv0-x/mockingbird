// Command mockingbird is a drop-in Go replacement for CastleMock.
//
//	mockingbird [serve]                       start the server (default)
//	mockingbird import --castlemock <dir>     convert a CastleMock data dir into the native store
//	mockingbird version
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/duv0-x/mockingbird/internal/api"
	"github.com/duv0-x/mockingbird/internal/auth"
	"github.com/duv0-x/mockingbird/internal/config"
	"github.com/duv0-x/mockingbird/internal/importer/castlemock"
	"github.com/duv0-x/mockingbird/internal/mock/forward"
	"github.com/duv0-x/mockingbird/internal/mock/rest"
	"github.com/duv0-x/mockingbird/internal/mock/soap"
	"github.com/duv0-x/mockingbird/internal/service"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/storage/jsonfile"
	"github.com/duv0-x/mockingbird/internal/web"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mockingbird:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout io.Writer) error {
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	switch cmd {
	case "serve":
		return serve(cfg)
	case "import":
		return importCastleMock(cfg, args, stdout)
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	default:
		return fmt.Errorf("unknown command %q (want serve, import or version)", cmd)
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func openStore(ctx context.Context, cfg config.Config) (storage.Store, error) {
	store, err := jsonfile.Open(ctx, jsonfile.Options{
		DataDir:       cfg.DataDir,
		SeedDir:       cfg.SeedDir,
		SeedMode:      jsonfile.SeedMode(cfg.SeedMode),
		RestEventMax:  cfg.RestEventMax,
		SoapEventMax:  cfg.SoapEventMax,
		AdminPassword: cfg.AdminPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", cfg.DataDir, err)
	}
	return store, nil
}

func serve(cfg config.Config) error {
	logger := newLogger(cfg)
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	authSvc, err := auth.New(store.Users(), auth.Options{KeyFile: cfg.JWTKeyFile})
	if err != nil {
		return fmt.Errorf("init auth: %w", err)
	}
	fwd := forward.NewClient(forward.Options{Timeout: cfg.ForwardTimeout, TLSVerify: cfg.TLSVerify})
	svc := service.New(service.Deps{Config: cfg, Store: store, Version: version, Logger: logger})

	handler := newRouter(cfg, logger, mounts{
		rest: rest.NewEngine(rest.Deps{Store: store, Forward: fwd, ContextPath: cfg.ContextPath, Logger: logger}),
		soap: soap.NewEngine(soap.Deps{Store: store, Forward: fwd, ContextPath: cfg.ContextPath, Logger: logger}),
		auth: authSvc.Middleware(),
		api: func(g *gin.RouterGroup) {
			api.Register(g, api.Deps{Config: cfg, Service: svc, Auth: authSvc, Logger: logger})
		},
		web: func(g *gin.RouterGroup) {
			web.Register(g, web.Deps{Config: cfg, Service: svc, Auth: authSvc, Version: version, Logger: logger})
		},
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		logger.Info("mockingbird started", "version", version, "port", cfg.Port, "contextPath", cfg.ContextPath, "dataDir", cfg.DataDir)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

type mounts struct {
	rest, soap http.Handler
	auth       gin.HandlerFunc
	api, web   func(*gin.RouterGroup)
}

// newRouter builds the gin engine (SPEC.md §7.2). Mock engines are plain
// http.Handlers mounted on catch-all routes and receive the unmodified path;
// they run without the auth middleware.
func newRouter(cfg config.Config, logger *slog.Logger, m mounts) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestLog(logger))

	health := func(c *gin.Context) { c.String(http.StatusOK, "ok\n") }
	r.GET("/healthz", health)
	r.GET("/readyz", health)

	ctx := cfg.ContextPath
	r.Any(ctx+"/mock/rest/*path", gin.WrapH(m.rest))
	r.Any(ctx+"/mock/soap/*path", gin.WrapH(m.soap))

	g := r.Group(ctx, m.auth)
	m.api(g)
	m.web(g)
	if ctx != "" {
		r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, ctx+"/") })
	}
	return r
}

func requestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Debug("request", "method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(), "duration", time.Since(start))
	}
}

func importCastleMock(cfg config.Config, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	src := fs.String("castlemock", "", "CastleMock data dir (base.file.directory, e.g. ~/.castlemock)")
	dataDir := fs.String("data-dir", cfg.DataDir, "target mockingbird data dir")
	overwrite := fs.Bool("overwrite", false, "replace projects and users that already exist in the target")
	skipEvents := fs.Bool("skip-events", false, "do not import recorded events")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *src == "" {
		return errors.New("import: --castlemock <dir> is required")
	}
	cfg.DataDir = *dataDir
	cfg.SeedDir = ""
	ctx := context.Background()
	store, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	report, err := castlemock.ImportDataDir(ctx, *src, store, castlemock.ImportOptions{Overwrite: *overwrite, SkipEvents: *skipEvents})
	if err != nil {
		return fmt.Errorf("import %s: %w", *src, err)
	}
	fmt.Fprintf(stdout, "imported %d REST projects, %d SOAP projects, %d users, %d REST events, %d SOAP events into %s\n",
		report.RestProjects, report.SoapProjects, report.Users, report.RestEvents, report.SoapEvents, cfg.DataDir)
	for _, w := range report.Warnings {
		fmt.Fprintln(stdout, "warning:", w)
	}
	for _, s := range report.Skipped {
		fmt.Fprintln(stdout, "skipped:", s)
	}
	return nil
}
