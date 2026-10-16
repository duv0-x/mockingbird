package castlemock_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/importer/castlemock"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/storage/jsonfile"
)

var update = flag.Bool("update", false, "rewrite golden files")

const fixtureHome = "testdata/castlemock-home"

func openStore(t *testing.T, dir string) storage.Store {
	t.Helper()
	s, err := jsonfile.Open(context.Background(), jsonfile.Options{DataDir: dir, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func importFixture(t *testing.T, opts castlemock.ImportOptions) (string, storage.Store, castlemock.Report) {
	t.Helper()
	dir := t.TempDir()
	s := openStore(t, dir)
	t.Cleanup(func() { s.Close() })
	rep, err := castlemock.ImportDataDir(context.Background(), fixtureHome, s, opts)
	if err != nil {
		t.Fatal(err)
	}
	return dir, s, rep
}

func containsLike(list []string, sub string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.Contains(s, sub) })
}

func TestImportDataDirReport(t *testing.T) {
	_, _, rep := importFixture(t, castlemock.ImportOptions{})
	if rep.RestProjects != 1 || rep.SoapProjects != 1 || rep.Users != 2 || rep.RestEvents != 1 || rep.SoapEvents != 1 {
		t.Fatalf("report %+v", rep)
	}
	for _, want := range []string{"Rs99zz", "broken.rest"} {
		if !containsLike(rep.Skipped, want) {
			t.Errorf("Skipped lacks %s: %q", want, rep.Skipped)
		}
	}
	for _, want := range []string{"rest/project/v1", "Rc03im"} {
		if !containsLike(rep.Warnings, want) {
			t.Errorf("Warnings lack %s: %q", want, rep.Warnings)
		}
	}
}

// TestImportDataDirGolden compares the native data dir produced from the
// CastleMock fixture with testdata/castlemock-home-expected.
func TestImportDataDirGolden(t *testing.T) {
	dir, s, _ := importFixture(t, castlemock.ImportOptions{})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	const expected = "testdata/castlemock-home-expected"
	got := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "meta.json" || strings.HasPrefix(rel, "secrets") {
			return nil
		}
		data, err := os.ReadFile(path)
		got[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.RemoveAll(expected); err != nil {
			t.Fatal(err)
		}
		for rel, data := range got {
			p := filepath.Join(expected, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := map[string][]byte{}
	err = filepath.WalkDir(expected, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(expected, path)
		data, err := os.ReadFile(path)
		want[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for rel, w := range want {
		if !bytes.Equal(got[rel], w) {
			t.Errorf("%s differs (run go test -update)\n--- got\n%s\n--- want\n%s", rel, got[rel], w)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("unexpected file %s", rel)
		}
	}
}

func TestImportDataDirContent(t *testing.T) {
	ctx := context.Background()
	dir, s, _ := importFixture(t, castlemock.ImportOptions{})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openStore(t, dir) // everything must survive a restart
	defer s.Close()

	admin, err := s.Users().GetByUsername(ctx, "ADMIN")
	if err != nil || admin.ID != "Ad01mn" || bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("legacy-admin")) != nil {
		t.Fatalf("imported admin did not replace the default one: %+v %v", admin, err)
	}
	users, _ := s.Users().List(ctx)
	if len(users) != 2 {
		t.Fatalf("users %d", len(users))
	}
	p, err := s.Rest().Get(ctx, "Pr01ab")
	if err != nil {
		t.Fatal(err)
	}
	m := p.Applications[0].Resources[0].Methods
	if len(m) != 2 || len(m[0].MockResponses) != 2 {
		t.Fatalf("tree %+v", p)
	}
	mr := m[0].MockResponses[0]
	if !strings.Contains(mr.Body, `"doggie & co"`) || !mr.ParameterQueries[0].URLEncoded || mr.HeaderQueries[0].Header != "X-Env" ||
		mr.JSONPathExpressions[0].Expression != "$.pet[?(@.id == 1)]" || mr.ContentEncodings[0] != domain.EncodingGZIP {
		t.Fatalf("mock response %+v", mr)
	}
	if seq, _ := s.State().SequenceIndex(ctx, "Me01gh"); seq != 2 {
		t.Fatalf("sequence index %d", seq)
	}
	sp, err := s.Soap().Get(ctx, "Sp01ab")
	if err != nil {
		t.Fatal(err)
	}
	ops := sp.Ports[0].Operations
	if ops[1].OperationIdentifier.Name != "SubtractRequest" || ops[1].IdentifyStrategy != domain.IdentifyByElementNamespace {
		t.Fatalf("legacy identifier: %+v", ops[1].OperationIdentifier)
	}
	if seq, _ := s.State().SequenceIndex(ctx, "Op02ij"); seq != 1 {
		t.Fatalf("soap sequence index %d", seq)
	}
	if c, err := s.Soap().ResourceContent(ctx, "Sp01ab", "Rc02sc"); err != nil || !strings.HasPrefix(c, "<xs:schema") {
		t.Fatalf("schema content %q %v", c, err)
	}
	ev, err := s.Events().GetRest(ctx, "Ev01re")
	if err != nil {
		t.Fatal(err)
	}
	// RestRequestFile swaps headers and parameters; the importer swaps them back.
	if ev.Request.HTTPHeaders[0].Name != "Accept" || ev.Request.HTTPParameters[0].Name != "verbose" {
		t.Fatalf("request %+v", ev.Request)
	}
	if sev, err := s.Events().GetSoap(ctx, "Ev02so"); err != nil || sev.Response != nil {
		t.Fatalf("soap event %+v %v", sev, err)
	}
}

func TestImportDataDirOptions(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		first castlemock.ImportOptions
		again castlemock.ImportOptions
		check func(t *testing.T, rep castlemock.Report)
	}{
		{"second import skips existing", castlemock.ImportOptions{}, castlemock.ImportOptions{}, func(t *testing.T, rep castlemock.Report) {
			if rep.RestProjects != 0 || rep.SoapProjects != 0 || rep.Users != 0 || !containsLike(rep.Skipped, "Pr01ab") {
				t.Fatalf("report %+v", rep)
			}
		}},
		{"overwrite replaces", castlemock.ImportOptions{}, castlemock.ImportOptions{Overwrite: true, SkipEvents: true}, func(t *testing.T, rep castlemock.Report) {
			if rep.RestProjects != 1 || rep.SoapProjects != 1 || rep.Users != 2 || rep.RestEvents != 0 {
				t.Fatalf("report %+v", rep)
			}
		}},
		{"skip events", castlemock.ImportOptions{SkipEvents: true}, castlemock.ImportOptions{SkipEvents: true}, func(t *testing.T, rep castlemock.Report) {
			if rep.RestEvents != 0 || rep.SoapEvents != 0 {
				t.Fatalf("report %+v", rep)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, s, first := importFixture(t, tt.first)
			if tt.first.SkipEvents && first.RestEvents != 0 {
				t.Fatalf("events imported: %+v", first)
			}
			rep, err := castlemock.ImportDataDir(ctx, fixtureHome, s, tt.again)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, rep)
		})
	}
}

func TestImportKeepsCustomizedAdmin(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openStore(t, dir)
	defer s.Close()
	admin, _ := s.Users().GetByUsername(ctx, "admin")
	hash, _ := bcrypt.GenerateFromPassword([]byte("changed"), bcrypt.MinCost)
	admin.Password = string(hash)
	if err := s.Users().Save(ctx, admin); err != nil {
		t.Fatal(err)
	}
	rep, err := castlemock.ImportDataDir(ctx, fixtureHome, s, castlemock.ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Users != 1 || !containsLike(rep.Skipped, "Ad01mn") {
		t.Fatalf("report %+v", rep)
	}
}

// exportFixture returns the imported fixture projects with the computed
// fields the service fills before exporting.
func exportFixture(t *testing.T) (domain.RestProject, domain.SoapProject, map[string]string) {
	t.Helper()
	ctx := context.Background()
	_, s, _ := importFixture(t, castlemock.ImportOptions{SkipEvents: true})
	rp, err := s.Rest().Get(ctx, "Pr01ab")
	if err != nil {
		t.Fatal(err)
	}
	for i := range rp.Applications {
		a := &rp.Applications[i]
		a.StatusCount = domain.NewRestStatusCount()
		for j := range a.Resources {
			r := &a.Resources[j]
			r.StatusCount = domain.NewRestStatusCount()
			for k := range r.Methods {
				m := &r.Methods[k]
				a.StatusCount[m.Status]++
				r.StatusCount[m.Status]++
				m.URI = r.URI
				m.CurrentResponseSequenceIndex, _ = s.State().SequenceIndex(ctx, m.ID)
			}
		}
	}
	sp, err := s.Soap().Get(ctx, "Sp01ab")
	if err != nil {
		t.Fatal(err)
	}
	for i := range sp.Ports {
		sp.Ports[i].StatusCount = domain.NewSoapStatusCount()
		for _, o := range sp.Ports[i].Operations {
			sp.Ports[i].StatusCount[o.Status]++
		}
	}
	contents := map[string]string{}
	for _, r := range sp.Resources {
		if c, err := s.Soap().ResourceContent(ctx, sp.ID, r.ID); err == nil {
			contents[r.ID] = c
		}
	}
	return rp, sp, contents
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
		t.Fatalf("%s differs (run go test -update)\n--- got\n%s", path, got)
	}
}

// stripComputed clears the fields a parsed export carries but the store
// never keeps, to compare trees.
func stripRest(p domain.RestProject) domain.RestProject {
	for i := range p.Applications {
		p.Applications[i].StatusCount = nil
		for j := range p.Applications[i].Resources {
			p.Applications[i].Resources[j].StatusCount = nil
			for k := range p.Applications[i].Resources[j].Methods {
				m := &p.Applications[i].Resources[j].Methods[k]
				m.URI, m.CurrentResponseSequenceIndex, m.DefaultResponseName = "", 0, ""
			}
		}
	}
	return p
}

func stripSoap(p domain.SoapProject) domain.SoapProject {
	p.StatusCount = nil
	for i := range p.Ports {
		p.Ports[i].StatusCount = nil
		for j := range p.Ports[i].Operations {
			p.Ports[i].Operations[j].CurrentResponseSequenceIndex = 0
		}
	}
	for i := range p.Resources {
		p.Resources[i].Content = ""
	}
	return p
}

func TestRestExportRoundTrip(t *testing.T) {
	rp, _, _ := exportFixture(t)
	data, err := castlemock.ExportRestProject(rp)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/export/rest-export.xml", data)
	parsed, err := castlemock.ParseRestExport(data)
	if err != nil {
		t.Fatal(err)
	}
	if m := parsed.Applications[0].Resources[0].Methods[0]; m.CurrentResponseSequenceIndex != 2 {
		t.Fatalf("sequence index not exported: %d", m.CurrentResponseSequenceIndex)
	}
	if again, _ := castlemock.ExportRestProject(withRestComputed(parsed, rp)); !bytes.Equal(again, data) {
		t.Fatalf("export(parse(export)) differs:\n%s", again)
	}
	// strip* clear computed fields in place, so they run last.
	if !sameJSON(t, stripRest(parsed), stripRest(rp)) {
		t.Fatal("round trip differs")
	}
}

func withRestComputed(p, from domain.RestProject) domain.RestProject {
	for i := range p.Applications {
		p.Applications[i].StatusCount = from.Applications[i].StatusCount
		for j := range p.Applications[i].Resources {
			p.Applications[i].Resources[j].StatusCount = from.Applications[i].Resources[j].StatusCount
		}
	}
	return p
}

func TestSoapExportRoundTrip(t *testing.T) {
	_, sp, contents := exportFixture(t)
	data, err := castlemock.ExportSoapProject(sp, contents)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/export/soap-export.xml", data)
	parsed, gotContents, err := castlemock.ParseSoapExport(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotContents, contents) {
		t.Fatalf("contents %q, want %q", gotContents, contents)
	}
	if !sameJSON(t, stripSoap(parsed), stripSoap(sp)) {
		t.Fatal("round trip differs")
	}
}

// TestParseExportVariants checks the tolerance rules of §6.3: single
// wrapping, ISO dates, any element order, unknown elements.
func TestParseExportVariants(t *testing.T) {
	canonical, err := castlemock.ParseRestExport(mustRead(t, "testdata/export/rest-export.xml"))
	if err != nil {
		t.Fatal(err)
	}
	variant, err := castlemock.ParseRestExport(mustRead(t, "testdata/export/rest-export-variant.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(t, stripRest(variant), stripRest(canonical)) {
		t.Fatal("variant differs")
	}

	tests := []struct {
		name, xml string
	}{
		{"wrong root", "<SoapExportContainer/>"},
		{"no project", "<RestExportContainer/>"},
		{"orphan resource", `<RestExportContainer><project><id>P</id><name>n</name></project>
			<resources><resources><id>R</id><applicationId>nope</applicationId></resources></resources></RestExportContainer>`},
		{"not xml", "{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := castlemock.ParseRestExport([]byte(tt.xml)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// sameJSON compares values by their API JSON, which ignores time.Time
// internals and nil-vs-empty maps.
func sameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ja, jb) {
		i := 0
		for i < len(ja) && i < len(jb) && ja[i] == jb[i] {
			i++
		}
		lo := max(0, i-80)
		t.Logf("JSON differs at %d:\ngot  …%s\nwant …%s", i, ja[lo:min(len(ja), i+80)], jb[lo:min(len(jb), i+80)])
		return false
	}
	return true
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
