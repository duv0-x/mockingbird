package main

import "testing"

// TestEndToEnd is the SPEC.md §10 rule 4 acceptance test, run at the
// integration step once T1–T6 are merged. It must:
//
//   - boot the server (runServe on a temp data dir) with MOCKINGBIRD_SEED_DIR
//     pointing at a seed dir fixture;
//   - log in through the admin API (POST /api/rest/core/login) and keep the
//     token cookie;
//   - create a REST project/application/resource/method/mock response through
//     the API and call the mock URL, asserting the response;
//   - render the main UI pages ({ctx}/web, project screen) with status 200;
//   - import a CastleMock data dir fixture (cmd `import --castlemock`) and
//     serve one of its SOAP mocks (`...?wsdl` and an operation POST).
//
// Kept skipped until then so the contract is visible in `go test -run E2E`.
func TestEndToEnd(t *testing.T) {
	t.Skip("e2e: deferred to the integration step after T1–T6 (SPEC.md §10 rule 4)")
}
