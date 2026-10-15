package domain

// Event is the common part of RestEvent and SoapEvent.
type Event struct {
	ID           string `json:"id"`
	ResourceName string `json:"resourceName"`
	StartDate    Time   `json:"startDate"`
	EndDate      Time   `json:"endDate"`
}

// OverviewEvent is an element of GET /api/rest/core/event.
type OverviewEvent struct {
	Event
	Type ProjectType `json:"type"`
}
