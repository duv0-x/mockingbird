package domain

// SystemInformation is the body of GET /api/rest/core/system. Field names are
// kept from CastleMock (Java/Tomcat oriented) because the frontend reads them;
// mockingbird fills them with Go runtime equivalents (SPEC.md §5.3).
type SystemInformation struct {
	OperatingSystemName         string `json:"operatingSystemName"`
	JavaVersion                 string `json:"javaVersion"`
	JavaVendor                  string `json:"javaVendor"`
	TomcatBuilt                 string `json:"tomcatBuilt"`
	TomcatInfo                  string `json:"tomcatInfo"`
	TomcatVersion               string `json:"tomcatVersion"`
	TotalMemory                 int64  `json:"totalMemory"`
	MaxMemory                   int64  `json:"maxMemory"`
	FreeMemory                  int64  `json:"freeMemory"`
	AvailableProcessors         int    `json:"availableProcessors"`
	CastleMockHomeDirectory     string `json:"castleMockHomeDirectory"`
	ShowCastleMockHomeDirectory bool   `json:"showCastleMockHomeDirectory"`
}
