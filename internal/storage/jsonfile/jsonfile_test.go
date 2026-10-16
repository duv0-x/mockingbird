package jsonfile_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/service/servicetest"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/storage/jsonfile"
)

var update = flag.Bool("update", false, "rewrite golden files")

func open(t *testing.T, opts jsonfile.Options) storage.Store {
	t.Helper()
	s, err := jsonfile.Open(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs (run go test -update)\n%s", path, got)
	}
}

func TestNativeFormatGolden(t *testing.T) {
	at := domain.Time{Time: time.Date(2024, 5, 1, 10, 20, 30, 123_000_000, time.UTC)}
	rp := servicetest.ContractRestProject("Ab12Cd", "x")
	rp.Created, rp.Updated, rp.Description = at, at, "Pets <&> more"
	rp.Applications[0].Resources[0].Methods[0].MockResponses[0].ContentEncodings = []domain.ContentEncoding{domain.EncodingGZIP}
	data, err := jsonfile.EncodeRestProject(rp)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/rest_project.json", data)

	sp := servicetest.ContractSoapProject("Ef34Gh", "y")
	sp.Created, sp.Updated = at, at
	data, err = jsonfile.EncodeSoapProject(sp, map[string]string{"yrsc1": "<definitions/>"})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/soap_project.json", data)

	// Decoding the golden files gives back the same trees.
	back, err := jsonfile.DecodeRestProject(mustRead(t, "testdata/rest_project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := jsonfile.EncodeRestProject(back); !bytes.Equal(again, mustRead(t, "testdata/rest_project.json")) {
		t.Fatalf("rest round trip differs:\n%s", again)
	}
	sback, contents, err := jsonfile.DecodeSoapProject(mustRead(t, "testdata/soap_project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if contents["yrsc1"] != "<definitions/>" || sback.Resources[0].Content != "" {
		t.Fatalf("contents %v", contents)
	}
	if again, _ := jsonfile.EncodeSoapProject(sback, contents); !bytes.Equal(again, mustRead(t, "testdata/soap_project.json")) {
		t.Fatalf("soap round trip differs:\n%s", again)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestDecodeSeedShapes covers hand-written seeds: missing ids and optional
// fields, expressions as objects, wrong kind.
func TestDecodeSeedShapes(t *testing.T) {
	minimal := `{"kind":"RestProject","name":"Seeded","applications":[{"name":"a","resources":[{"name":"r","uri":"/r",
		"methods":[{"name":"m","mockResponses":[{"name":"ok","xpathExpressions":[{"expression":"//a"}],"jsonPathExpressions":["$.a"]}]}]}]}]}`
	p, err := jsonfile.DecodeRestProject([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	m := p.Applications[0].Resources[0].Methods[0]
	mr := m.MockResponses[0]
	if p.ID == "" || m.ID == "" || mr.ID == "" || m.ResourceID != p.Applications[0].Resources[0].ID {
		t.Fatalf("ids not generated: %+v", p)
	}
	if m.HTTPMethod != domain.MethodGET || m.Status != domain.RestMethodMocked || mr.HTTPStatusCode != 200 || mr.Status != domain.RestMockResponseEnabled {
		t.Fatalf("defaults %+v", m)
	}
	if mr.XPathExpressions[0].Expression != "//a" || mr.JSONPathExpressions[0].Expression != "$.a" {
		t.Fatalf("expressions %+v", mr)
	}
	if _, err := jsonfile.DecodeRestProject([]byte(`{"kind":"SoapProject","name":"x"}`)); err == nil {
		t.Fatal("wrong kind accepted")
	}
	if _, _, err := jsonfile.DecodeSoapProject([]byte(`{"kind":"SoapProject","schemaVersion":2}`)); err == nil {
		t.Fatal("future schema accepted")
	}
}

func TestPersistenceAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := open(t, jsonfile.Options{DataDir: dir, AdminPassword: "s3cret", Version: "1.0"})
	if err := s.Rest().Create(ctx, servicetest.ContractRestProject("P1", "a")); err != nil {
		t.Fatal(err)
	}
	if err := s.Soap().Create(ctx, servicetest.ContractSoapProject("S1", "b")); err != nil {
		t.Fatal(err)
	}
	if err := s.Soap().PutResourceContent(ctx, "S1", "brsc1", "<wsdl/>"); err != nil {
		t.Fatal(err)
	}
	if err := s.Events().AddRest(ctx, domain.RestEvent{Event: domain.Event{ID: "E1", StartDate: domain.Now()}}); err != nil {
		t.Fatal(err)
	}
	if err := s.State().SetSequenceIndex(ctx, "amet1", 4); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{"meta.json", "users.json", "projects/rest/P1.json", "projects/soap/S1.json",
		"projects/soap/S1/resources/brsc1.wsdl", "events/rest/E1.json", "state/sequence.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	if st, _ := os.Stat(filepath.Join(dir, "secrets")); st == nil || st.Mode().Perm() != 0o700 {
		t.Errorf("secrets dir mode %v", st)
	}
	if meta := string(mustRead(t, filepath.Join(dir, "meta.json"))); !strings.Contains(meta, `"createdBy": "mockingbird 1.0"`) {
		t.Errorf("meta %s", meta)
	}

	s = open(t, jsonfile.Options{DataDir: dir})
	defer s.Close()
	p, err := s.Rest().Get(ctx, "P1")
	if err != nil || *p.Applications[0].Resources[0].Methods[0].NetworkDelay != 5 {
		t.Fatalf("project after restart: %+v %v", p, err)
	}
	if pid, _ := s.Rest().ProjectOf(ctx, "amr1"); pid != "P1" {
		t.Fatal("index not rebuilt")
	}
	if c, _ := s.Soap().ResourceContent(ctx, "S1", "brsc1"); c != "<wsdl/>" {
		t.Fatalf("content %q", c)
	}
	if _, err := s.Events().GetRest(ctx, "E1"); err != nil {
		t.Fatal(err)
	}
	if i, _ := s.State().SequenceIndex(ctx, "amet1"); i != 4 {
		t.Fatalf("sequence %d", i)
	}
	users, _ := s.Users().List(ctx)
	if len(users) != 1 || users[0].Username != "admin" || users[0].FullName != "Admin Admin" || users[0].Role != domain.RoleAdmin ||
		bcrypt.CompareHashAndPassword([]byte(users[0].Password), []byte("s3cret")) != nil {
		t.Fatalf("default admin %+v", users)
	}
}

func TestStateIsWrittenBehind(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := open(t, jsonfile.Options{DataDir: dir})
	defer s.Close()
	_ = s.State().SetSequenceIndex(ctx, "k", 1)
	path := filepath.Join(dir, "state", "sequence.json")
	deadline := time.Now().Add(3 * time.Second)
	for {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), `"k": 1`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("sequence state not flushed within 3s")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestUnsupportedSchemaRefusesToStart(t *testing.T) {
	for _, f := range []string{"users.json", "projects/rest/P1.json", "meta.json"} {
		t.Run(f, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, f)
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte(`{"schemaVersion": 2, "kind": "RestProject", "id": "P1"}`), 0o644)
			if _, err := jsonfile.Open(context.Background(), jsonfile.Options{DataDir: dir}); err == nil || !strings.Contains(err.Error(), "schemaVersion") {
				t.Fatalf("Open: %v", err)
			}
		})
	}
}

func TestCorruptFilesAreSkipped(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "projects", "rest"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "projects", "rest", "Bad.json"), []byte("{not json"), 0o644)
	s := open(t, jsonfile.Options{DataDir: dir})
	defer s.Close()
	if list, _ := s.Rest().List(ctx); len(list) != 0 {
		t.Fatalf("list %v", list)
	}
	if data := mustRead(t, filepath.Join(dir, "projects", "rest", "Bad.json")); string(data) != "{not json" {
		t.Fatal("corrupt file was modified")
	}
}

func TestEventEviction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := open(t, jsonfile.Options{DataDir: dir, RestEventMax: 2, SoapEventMax: 1})
	defer s.Close()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"B", "A", "C"} { // A is the oldest
		offset := map[string]int{"A": 0, "B": 1, "C": 2}[id]
		ev := domain.RestEvent{Event: domain.Event{ID: id, StartDate: domain.Time{Time: base.Add(time.Duration(offset) * time.Minute)}}}
		if err := s.Events().AddRest(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := s.Events().ListRest(ctx)
	if len(list) != 2 {
		t.Fatalf("len %d", len(list))
	}
	if _, err := s.Events().GetRest(ctx, "A"); err == nil {
		t.Fatal("oldest event not evicted")
	}
	if _, err := os.Stat(filepath.Join(dir, "events", "rest", "A.json")); !os.IsNotExist(err) {
		t.Fatal("evicted event file not deleted")
	}
}

func writeSeed(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSeeding(t *testing.T) {
	ctx := context.Background()
	restSeed := func(name string) string {
		return `{"schemaVersion":1,"kind":"RestProject","id":"Seed01","name":"` + name + `","applications":[]}`
	}
	soapSeed := `{"schemaVersion":1,"kind":"SoapProject","id":"Seed02","name":"Soap seed",
		"resources":[{"id":"SeedR1","name":"a.wsdl","type":"WSDL","content":"<definitions/>"}],"ports":[]}`

	seeds := t.TempDir()
	writeSeed(t, seeds, "rest.json", restSeed("First"))
	writeSeed(t, seeds, "nested/soap.json", soapSeed)
	writeSeed(t, seeds, "..2024_data/rest.json", restSeed("Duplicate in ConfigMap dir"))
	writeSeed(t, seeds, "README.md", "ignored")
	writeSeed(t, seeds, "export/rest-export.xml", string(mustRead(t, "../../importer/castlemock/testdata/export/rest-export.xml")))
	if err := os.Symlink(filepath.Join(seeds, "nested"), filepath.Join(seeds, "linked")); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	s := open(t, jsonfile.Options{DataDir: dir, SeedDir: seeds})
	rest, _ := s.Rest().List(ctx)
	soap, _ := s.Soap().List(ctx)
	if len(rest) != 2 || len(soap) != 1 {
		t.Fatalf("seeded %d rest, %d soap", len(rest), len(soap))
	}
	if c, err := s.Soap().ResourceContent(ctx, "Seed02", "SeedR1"); err != nil || c != "<definitions/>" {
		t.Fatalf("seed content %q %v", c, err)
	}
	if _, err := s.Rest().Get(ctx, "Pr01ab"); err != nil {
		t.Fatalf("XML export seed not loaded: %v", err)
	}
	// A UI edit survives a restart in create-only mode.
	_, _ = s.Rest().Update(ctx, "Seed01", func(p *domain.RestProject) error { p.Name = "Edited"; return nil })
	_ = s.Close()

	writeSeed(t, seeds, "rest.json", restSeed("Changed seed"))
	s = open(t, jsonfile.Options{DataDir: dir, SeedDir: seeds})
	if p, _ := s.Rest().Get(ctx, "Seed01"); p.Name != "Edited" {
		t.Fatalf("create-only overwrote: %q", p.Name)
	}
	_ = s.Close()

	s = open(t, jsonfile.Options{DataDir: dir, SeedDir: seeds, SeedMode: jsonfile.SeedOverwrite})
	if p, _ := s.Rest().Get(ctx, "Seed01"); p.Name != "Changed seed" {
		t.Fatalf("overwrite did not apply: %q", p.Name)
	}
	_ = s.Close()

	// Seeds are never written.
	if got := string(mustRead(t, filepath.Join(seeds, "rest.json"))); got != restSeed("Changed seed") {
		t.Fatal("seed file modified")
	}
}

func TestSeedNameConflictIsSkipped(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := open(t, jsonfile.Options{DataDir: dir})
	_ = s.Rest().Create(ctx, domain.RestProject{Project: domain.Project{ID: "Own001", Name: "Taken"}})
	_ = s.Close()
	seeds := t.TempDir()
	writeSeed(t, seeds, "p.json", `{"kind":"RestProject","id":"Seed01","name":"TAKEN"}`)
	s = open(t, jsonfile.Options{DataDir: dir, SeedDir: seeds})
	defer s.Close()
	if _, err := s.Rest().Get(ctx, "Seed01"); err == nil {
		t.Fatal("seed with a taken name was added")
	}
}

func TestBadSeedAbortsStartup(t *testing.T) {
	tests := map[string]string{
		"broken.json":   "{",
		"unknown.json":  `{"kind":"Something"}`,
		"broken.xml":    "<RestExportContainer>",
		"wrongroot.xml": "<html/>",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			seeds := t.TempDir()
			writeSeed(t, seeds, name, content)
			if _, err := jsonfile.Open(context.Background(), jsonfile.Options{DataDir: t.TempDir(), SeedDir: seeds}); err == nil {
				t.Fatal("startup did not fail")
			}
		})
	}
}
