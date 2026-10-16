package servicetest

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// StoreContract runs the storage.Store contract (SPEC.md §3.3) against
// stores returned by open, which must be empty of projects and events (it
// may hold users, e.g. the default admin). Both MemStore and the jsonfile
// store run it, so tests written against MemStore stay meaningful.
func StoreContract(t *testing.T, open func(t *testing.T) storage.Store) {
	t.Run("rest", func(t *testing.T) { restContract(t, open(t)) })
	t.Run("soap", func(t *testing.T) { soapContract(t, open(t)) })
	t.Run("cross-kind ids", func(t *testing.T) { crossKindContract(t, open(t)) })
	t.Run("users", func(t *testing.T) { usersContract(t, open(t)) })
	t.Run("events", func(t *testing.T) { eventsContract(t, open(t)) })
	t.Run("state", func(t *testing.T) { stateContract(t, open(t)) })
}

// ContractRestProject is a small full tree with fixed ids.
func ContractRestProject(id, prefix string) domain.RestProject {
	return domain.RestProject{
		Project: domain.Project{ID: id, Name: "Project " + id, Created: domain.Now(), Updated: domain.Now()},
		Applications: []domain.RestApplication{{ID: prefix + "app1", Name: "app", Resources: []domain.RestResource{{
			ID: prefix + "res1", Name: "pet", URI: "/pet/{id}", Methods: []domain.RestMethod{{
				ID: prefix + "met1", Name: "get", HTTPMethod: domain.MethodGET, Status: domain.RestMethodMocked,
				ResponseStrategy: domain.RestStrategyRandom, SimulateNetworkDelay: domain.Ptr(true), NetworkDelay: domain.Ptr(int64(5)),
				MockResponses: []domain.RestMockResponse{{
					ID: prefix + "mr1", Name: "ok", Body: "{}", HTTPStatusCode: 200, Status: domain.RestMockResponseEnabled,
					UsingExpressions:    domain.Ptr(false),
					HTTPHeaders:         []domain.HTTPHeader{{Name: "Content-Type", Value: "application/json"}},
					XPathExpressions:    []domain.RestXPathExpression{{Expression: "//a"}},
					JSONPathExpressions: []domain.RestJSONPathExpression{{Expression: "$.a"}},
				}},
			}},
		}}}},
	}
}

// ContractSoapProject is a small full tree with fixed ids and one resource.
func ContractSoapProject(id, prefix string) domain.SoapProject {
	return domain.SoapProject{
		Project:   domain.Project{ID: id, Name: "Soap " + id, Created: domain.Now(), Updated: domain.Now()},
		Resources: []domain.SoapResource{{ID: prefix + "rsc1", Name: "calc.wsdl", Type: domain.SoapResourceWSDL}},
		Ports: []domain.SoapPort{{ID: prefix + "prt1", Name: "CalcSoap", URI: "CalcSoap", Operations: []domain.SoapOperation{{
			ID: prefix + "opr1", Name: "Add", OperationIdentifier: domain.SoapOperationIdentifier{Name: "Add", Namespace: "http://t/"},
			Status: domain.SoapOperationMocked, ResponseStrategy: domain.SoapStrategyRandom, HTTPMethod: domain.MethodPOST,
			SoapVersion: domain.SOAP11, IdentifyStrategy: domain.IdentifyByElementNamespace,
			MockResponses: []domain.SoapMockResponse{{ID: prefix + "smr1", Name: "r", Body: "<a/>", HTTPStatusCode: 200,
				Status: domain.SoapMockResponseEnabled, XPathExpressions: []domain.SoapXPathExpression{{Expression: "//a"}}}},
		}}}},
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("got error %v, want %v", err, target)
	}
}

func restContract(t *testing.T, s storage.Store) {
	ctx := context.Background()
	repo := s.Rest()
	check(t, repo.Create(ctx, ContractRestProject("P1", "a")))
	check(t, repo.Create(ctx, ContractRestProject("P2", "b")))
	wantErr(t, repo.Create(ctx, ContractRestProject("P1", "c")), storage.ErrConflict)
	wantErr(t, repo.Create(ctx, ContractRestProject("P3", "a")), storage.ErrConflict)

	list, err := repo.List(ctx)
	check(t, err)
	if len(list) != 2 || list[0].ID != "P1" || list[1].ID != "P2" {
		t.Fatalf("list %v", list)
	}
	p, err := repo.Get(ctx, "P1")
	check(t, err)
	m := p.Applications[0].Resources[0].Methods[0]
	if m.ResourceID != "ares1" || p.Applications[0].ProjectID != "P1" || m.MockResponses[0].MethodID != "amet1" {
		t.Fatalf("parent ids not set: %+v", m)
	}
	if *m.SimulateNetworkDelay != true || *m.NetworkDelay != 5 || m.MockResponses[0].XPathExpressions[0].Expression != "//a" {
		t.Fatalf("fields lost: %+v", m)
	}
	_, err = repo.Get(ctx, "nope")
	wantErr(t, err, storage.ErrNotFound)

	for _, id := range []string{"aapp1", "ares1", "amet1", "amr1"} {
		pid, err := repo.ProjectOf(ctx, id)
		check(t, err)
		if pid != "P1" {
			t.Fatalf("ProjectOf(%s) = %s", id, pid)
		}
	}
	_, err = repo.ProjectOf(ctx, "nope")
	wantErr(t, err, storage.ErrNotFound)

	found, err := repo.FindByName(ctx, "PROJECT p2")
	check(t, err)
	if found.ID != "P2" {
		t.Fatalf("FindByName %s", found.ID)
	}
	_, err = repo.FindByName(ctx, "missing")
	wantErr(t, err, storage.ErrNotFound)

	// Reads are snapshots.
	p.Name = "mutated"
	*p.Applications[0].Resources[0].Methods[0].SimulateNetworkDelay = false
	p.Applications[0].Resources[0].Methods[0].MockResponses[0].HTTPHeaders[0].Value = "x"
	again, _ := repo.Get(ctx, "P1")
	am := again.Applications[0].Resources[0].Methods[0]
	if again.Name == "mutated" || !*am.SimulateNetworkDelay || am.MockResponses[0].HTTPHeaders[0].Value == "x" {
		t.Fatal("Get returned shared state")
	}

	// Update: applied and persisted, children indexed; callers keeping
	// pointers into the callback tree cannot change the stored tree.
	var kept *domain.RestApplication
	updated, err := repo.Update(ctx, "P1", func(p *domain.RestProject) error {
		p.Applications = append(p.Applications, domain.RestApplication{ID: "aapp2", Name: "second"})
		kept = &p.Applications[1]
		return nil
	})
	check(t, err)
	kept.Name = "changed after update"
	if len(updated.Applications) != 2 || updated.Applications[1].ProjectID != "P1" {
		t.Fatalf("update result %+v", updated.Applications)
	}
	if got, _ := repo.Get(ctx, "P1"); got.Applications[1].Name != "second" {
		t.Fatalf("stored tree aliased by the callback: %q", got.Applications[1].Name)
	}
	if pid, err := repo.ProjectOf(ctx, "aapp2"); err != nil || pid != "P1" {
		t.Fatalf("new child not indexed: %v %v", pid, err)
	}

	// A failing callback changes nothing.
	boom := errors.New("boom")
	_, err = repo.Update(ctx, "P1", func(p *domain.RestProject) error {
		p.Name = "nope"
		return boom
	})
	wantErr(t, err, boom)
	if got, _ := repo.Get(ctx, "P1"); got.Name != "Project P1" {
		t.Fatal("failed update was applied")
	}
	_, err = repo.Update(ctx, "nope", func(*domain.RestProject) error { return nil })
	wantErr(t, err, storage.ErrNotFound)
	// Update cannot steal a child id of another project.
	_, err = repo.Update(ctx, "P1", func(p *domain.RestProject) error {
		p.Applications[1].ID = "bapp1"
		return nil
	})
	wantErr(t, err, storage.ErrConflict)

	// Removing a child drops it from the index.
	_, err = repo.Update(ctx, "P1", func(p *domain.RestProject) error {
		p.Applications = p.Applications[:1]
		return nil
	})
	check(t, err)
	_, err = repo.ProjectOf(ctx, "aapp2")
	wantErr(t, err, storage.ErrNotFound)

	// Put replaces.
	repl := ContractRestProject("P2", "z")
	repl.Name = "Replaced"
	check(t, repo.Put(ctx, repl))
	if got, _ := repo.Get(ctx, "P2"); got.Name != "Replaced" || got.Applications[0].ID != "zapp1" {
		t.Fatalf("put %+v", got)
	}
	_, err = repo.ProjectOf(ctx, "bapp1")
	wantErr(t, err, storage.ErrNotFound)

	deleted, err := repo.Delete(ctx, "P1")
	check(t, err)
	if deleted.ID != "P1" {
		t.Fatalf("deleted %s", deleted.ID)
	}
	_, err = repo.Get(ctx, "P1")
	wantErr(t, err, storage.ErrNotFound)
	_, err = repo.ProjectOf(ctx, "amet1")
	wantErr(t, err, storage.ErrNotFound)
	_, err = repo.Delete(ctx, "P1")
	wantErr(t, err, storage.ErrNotFound)
	// The ids of a deleted project can be reused.
	check(t, repo.Create(ctx, ContractRestProject("P1", "a")))
}

func soapContract(t *testing.T, s storage.Store) {
	ctx := context.Background()
	repo := s.Soap()
	check(t, repo.Create(ctx, ContractSoapProject("S1", "a")))
	wantErr(t, repo.Create(ctx, ContractSoapProject("S2", "a")), storage.ErrConflict)
	for _, id := range []string{"arsc1", "aprt1", "aopr1", "asmr1"} {
		if pid, err := repo.ProjectOf(ctx, id); err != nil || pid != "S1" {
			t.Fatalf("ProjectOf(%s) = %s, %v", id, pid, err)
		}
	}
	p, err := repo.Get(ctx, "S1")
	check(t, err)
	if p.Ports[0].ProjectID != "S1" || p.Ports[0].Operations[0].PortID != "aprt1" || p.Resources[0].ProjectID != "S1" {
		t.Fatalf("parent ids %+v", p.Ports[0])
	}
	if p.Ports[0].Operations[0].OperationIdentifier.Namespace != "http://t/" {
		t.Fatal("operation identifier lost")
	}

	_, err = repo.ResourceContent(ctx, "S1", "arsc1")
	wantErr(t, err, storage.ErrNotFound)
	wantErr(t, repo.PutResourceContent(ctx, "S1", "missing", "x"), storage.ErrNotFound)
	check(t, repo.PutResourceContent(ctx, "S1", "arsc1", "<definitions/>"))
	c, err := repo.ResourceContent(ctx, "S1", "arsc1")
	check(t, err)
	if c != "<definitions/>" {
		t.Fatalf("content %q", c)
	}
	// Removing the resource from the tree deletes its content.
	_, err = repo.Update(ctx, "S1", func(p *domain.SoapProject) error {
		p.Resources = nil
		return nil
	})
	check(t, err)
	_, err = repo.ResourceContent(ctx, "S1", "arsc1")
	wantErr(t, err, storage.ErrNotFound)

	found, err := repo.FindByName(ctx, "soap s1")
	check(t, err)
	if found.ID != "S1" {
		t.Fatal("FindByName")
	}
	check(t, repo.Put(ctx, ContractSoapProject("S2", "b")))
	check(t, repo.PutResourceContent(ctx, "S2", "brsc1", "<x/>"))
	_, err = repo.Delete(ctx, "S2")
	check(t, err)
	_, err = repo.ResourceContent(ctx, "S2", "brsc1")
	wantErr(t, err, storage.ErrNotFound)
	list, _ := repo.List(ctx)
	if len(list) != 1 {
		t.Fatalf("list %d", len(list))
	}
}

func crossKindContract(t *testing.T, s storage.Store) {
	ctx := context.Background()
	check(t, s.Rest().Create(ctx, ContractRestProject("X1", "x")))
	// A SOAP tree reusing a REST child id conflicts: ids are unique store-wide.
	sp := ContractSoapProject("Y1", "y")
	sp.Ports[0].ID = "xapp1"
	wantErr(t, s.Soap().Create(ctx, sp), storage.ErrConflict)
	wantErr(t, s.Soap().Create(ctx, ContractSoapProject("X1", "w")), storage.ErrConflict)
}

func usersContract(t *testing.T, s storage.Store) {
	ctx := context.Background()
	users := s.Users()
	before, err := users.List(ctx)
	check(t, err)
	u := domain.User{ID: "U1", Username: "Alice", Password: "$2a$10$x", Status: domain.UserActive, Role: domain.RoleReader,
		Created: domain.Now(), Updated: domain.Now()}
	check(t, users.Save(ctx, u))
	got, err := users.GetByUsername(ctx, "ALICE")
	check(t, err)
	if got.ID != "U1" || got.Password != "$2a$10$x" {
		t.Fatalf("user %+v", got)
	}
	u.Username = "alicia"
	check(t, users.Save(ctx, u))
	_, err = users.GetByUsername(ctx, "alice")
	wantErr(t, err, storage.ErrNotFound)
	if got, _ := users.Get(ctx, "U1"); got.Username != "alicia" {
		t.Fatal("rename not saved")
	}
	list, _ := users.List(ctx)
	if len(list) != len(before)+1 || !slices.ContainsFunc(list, func(x domain.User) bool { return x.ID == "U1" }) {
		t.Fatalf("list %d", len(list))
	}
	_, err = users.Delete(ctx, "U1")
	check(t, err)
	_, err = users.Get(ctx, "U1")
	wantErr(t, err, storage.ErrNotFound)
	_, err = users.GetByUsername(ctx, "alicia")
	wantErr(t, err, storage.ErrNotFound)
	_, err = users.Delete(ctx, "U1")
	wantErr(t, err, storage.ErrNotFound)
}

func eventsContract(t *testing.T, s storage.Store) {
	ctx := context.Background()
	ev := s.Events()
	re := domain.RestEvent{Event: domain.Event{ID: "E1", ResourceName: "get", StartDate: domain.Now(), EndDate: domain.Now()},
		Request:  domain.RestRequest{URI: "/a", HTTPMethod: domain.MethodGET, HTTPHeaders: []domain.HTTPHeader{{Name: "A", Value: "1"}}},
		Response: &domain.RestResponse{HTTPStatusCode: 200, Body: "ok"}, ProjectID: "p", MethodID: "m"}
	check(t, ev.AddRest(ctx, re))
	re.Request.HTTPHeaders[0].Value = "mutated"
	got, err := ev.GetRest(ctx, "E1")
	check(t, err)
	if got.Request.HTTPHeaders[0].Value != "1" || got.Response.Body != "ok" {
		t.Fatalf("event %+v", got)
	}
	check(t, ev.AddSoap(ctx, domain.SoapEvent{Event: domain.Event{ID: "E2", ResourceName: "Add", StartDate: domain.Now()}}))
	rl, _ := ev.ListRest(ctx)
	sl, _ := ev.ListSoap(ctx)
	if len(rl) != 1 || len(sl) != 1 {
		t.Fatalf("lists %d %d", len(rl), len(sl))
	}
	check(t, ev.ClearRest(ctx))
	_, err = ev.GetRest(ctx, "E1")
	wantErr(t, err, storage.ErrNotFound)
	if _, err := ev.GetSoap(ctx, "E2"); err != nil {
		t.Fatal("ClearRest removed SOAP events")
	}
	check(t, ev.ClearSoap(ctx))
	_, err = ev.GetSoap(ctx, "E2")
	wantErr(t, err, storage.ErrNotFound)
}

func stateContract(t *testing.T, s storage.Store) {
	ctx := context.Background()
	st := s.State()
	if i, err := st.SequenceIndex(ctx, "k"); err != nil || i != 0 {
		t.Fatalf("missing key = %d, %v", i, err)
	}
	check(t, st.SetSequenceIndex(ctx, "k", 3))
	if i, _ := st.SequenceIndex(ctx, "k"); i != 3 {
		t.Fatalf("index %d", i)
	}
	check(t, st.DeleteSequenceIndex(ctx, "k"))
	check(t, st.DeleteSequenceIndex(ctx, "never"))
	if i, _ := st.SequenceIndex(ctx, "k"); i != 0 {
		t.Fatalf("index after delete %d", i)
	}
	check(t, s.Close())
}
