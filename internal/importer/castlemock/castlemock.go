// Package castlemock reads every CastleMock legacy format (SPEC.md §3.6, §6.3):
//   - the XML data dir (~/.castlemock), converted once into the native store
//     by `mockingbird import --castlemock <dir>`;
//   - the project export files produced by GET .../project/{id}/export
//     (Jackson XmlMapper format), which it also writes for API exports.
//
// Both formats name every field like the Java model (JAXB in the data dir,
// Jackson getters in exports), so one set of element → domain converters
// (entities.go) serves both; only list wrapping and date encoding differ,
// and the readers accept either form.
//
// Owner: T1. Nothing at runtime depends on the legacy data dir.
package castlemock

type ImportOptions struct {
	// Overwrite replaces projects/users whose id already exists in the target store.
	Overwrite bool
	// SkipEvents ignores rest/event and soap/event.
	SkipEvents bool
	// AdminPassword is the target store's default admin password
	// (MOCKINGBIRD_ADMIN_PASSWORD); "" means "admin". An untouched default
	// admin in the target is replaced by the imported "admin" user.
	AdminPassword string
}

type Report struct {
	RestProjects int
	SoapProjects int
	Users        int
	RestEvents   int
	SoapEvents   int
	Skipped      []string
	Warnings     []string
}
