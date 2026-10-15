// Package config loads runtime configuration from environment variables
// (SPEC.md §7). Owner: T0 (contract); T6 may extend it.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        int
	ContextPath string // "/castlemock" by default; "" means served at root.
	DataDir     string
	SeedDir     string
	SeedMode    string // "create-only" | "overwrite"

	LogLevel  string // debug|info|warn|error
	LogFormat string // text|json

	RestEventMax int
	SoapEventMax int

	UploadMaxBytes int64
	TLSVerify      bool
	ForwardTimeout time.Duration

	AdminPassword string
	JWTKeyFile    string
	ShowDataDir   bool
}

func Default() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dataDir := filepath.Join(home, ".mockingbird")
	return Config{
		Port:           8080,
		ContextPath:    "/castlemock",
		DataDir:        dataDir,
		SeedMode:       "create-only",
		LogLevel:       "info",
		LogFormat:      "text",
		RestEventMax:   100,
		SoapEventMax:   100,
		UploadMaxBytes: 20 << 20,
		TLSVerify:      true,
		ForwardTimeout: 60 * time.Second,
		ShowDataDir:    true,
	}
}

// Load reads MOCKINGBIRD_* variables on top of Default. getenv is os.Getenv in production.
func Load(getenv func(string) string) (Config, error) {
	c := Default()
	var errs []string
	str := func(key string, dst *string) {
		if v := getenv(key); v != "" {
			*dst = v
		}
	}
	num := func(key string, dst *int) {
		if v := getenv(key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", key, err))
				return
			}
			*dst = n
		}
	}
	boolean := func(key string, dst *bool) {
		if v := getenv(key); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", key, err))
				return
			}
			*dst = b
		}
	}

	num("MOCKINGBIRD_PORT", &c.Port)
	if v, ok := lookup(getenv, "MOCKINGBIRD_CONTEXT_PATH"); ok {
		c.ContextPath = NormalizeContextPath(v)
	}
	str("MOCKINGBIRD_DATA_DIR", &c.DataDir)
	str("MOCKINGBIRD_SEED_DIR", &c.SeedDir)
	str("MOCKINGBIRD_SEED_MODE", &c.SeedMode)
	str("MOCKINGBIRD_LOG_LEVEL", &c.LogLevel)
	str("MOCKINGBIRD_LOG_FORMAT", &c.LogFormat)
	num("MOCKINGBIRD_REST_EVENT_MAX", &c.RestEventMax)
	num("MOCKINGBIRD_SOAP_EVENT_MAX", &c.SoapEventMax)
	boolean("MOCKINGBIRD_TLS_VERIFY", &c.TLSVerify)
	boolean("MOCKINGBIRD_SHOW_DATA_DIR", &c.ShowDataDir)
	str("MOCKINGBIRD_ADMIN_PASSWORD", &c.AdminPassword)
	str("MOCKINGBIRD_JWT_KEY_FILE", &c.JWTKeyFile)
	if v := getenv("MOCKINGBIRD_UPLOAD_MAX_SIZE"); v != "" {
		n, err := ParseSize(v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("MOCKINGBIRD_UPLOAD_MAX_SIZE: %v", err))
		}
		c.UploadMaxBytes = n
	}
	if v := getenv("MOCKINGBIRD_FORWARD_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("MOCKINGBIRD_FORWARD_TIMEOUT: %v", err))
		}
		c.ForwardTimeout = d
	}
	if c.JWTKeyFile == "" {
		c.JWTKeyFile = filepath.Join(c.DataDir, "secrets", "jwt.key")
	}
	if c.SeedMode != "create-only" && c.SeedMode != "overwrite" {
		errs = append(errs, fmt.Sprintf("MOCKINGBIRD_SEED_MODE: unsupported value %q", c.SeedMode))
	}
	if len(errs) > 0 {
		return c, fmt.Errorf("invalid configuration: %s", strings.Join(errs, "; "))
	}
	return c, nil
}

func lookup(getenv func(string) string, key string) (string, bool) {
	v := getenv(key)
	return v, v != ""
}

// NormalizeContextPath turns "castlemock", "/castlemock/" into "/castlemock" and "/" into "".
func NormalizeContextPath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// ParseSize accepts Spring-style sizes: "20MB", "512KB", "1GB" or plain bytes.
func ParseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	mult := int64(1)
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			s, mult = rest, u.mult
			break
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse size: %w", err)
	}
	return n * mult, nil
}
