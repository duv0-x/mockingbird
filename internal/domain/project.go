package domain

type ProjectType string

const (
	ProjectTypeREST ProjectType = "rest"
	ProjectTypeSOAP ProjectType = "soap"
)

// Project is the common part of RestProject and SoapProject
// (com.castlemock.model.core.project.Project).
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Updated     Time   `json:"updated"`
	Created     Time   `json:"created"`
	Description string `json:"description,omitempty"`
}

// OverviewProject is an element of GET /api/rest/core/project.
type OverviewProject struct {
	Project
	Type ProjectType `json:"type"`
}
