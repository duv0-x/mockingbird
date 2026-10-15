package api

import "net/http"

const (
	restProject   = "/api/rest/rest/project/:projectId"
	restApp       = restProject + "/application/:applicationId"
	restResource  = restApp + "/resource/:resourceId"
	restMethod    = restResource + "/method/:methodId"
	restMock      = restMethod + "/mockresponse/:mockResponseId"
	soapProject   = "/api/rest/soap/project/:projectId"
	soapPort      = soapProject + "/port/:portId"
	soapOperation = soapPort + "/operation/:operationId"
	soapMock      = soapOperation + "/mockresponse/:mockResponseId"
	soapResource  = soapProject + "/resource/:resourceId"
)

// Routes is the admin API (SPEC.md §5.3–5.5). Lookups use the last id of the
// path; parent ids are ignored like in CastleMock (§5.2).
func Routes() []Route {
	return []Route{
		// Core (§5.3).
		{http.MethodPost, "/api/rest/core/login", Public, "Log in and receive a session token", (*handlers).login},
		{http.MethodGet, "/api/rest/core/logout", Public, "Clear the session cookies", (*handlers).logout},
		{http.MethodGet, "/api/rest/core/version", Public, "Server version", (*handlers).version},
		{http.MethodGet, "/api/rest/core/context", Public, "Context path", (*handlers).context},
		{http.MethodGet, "/api/rest/core/profile", Reader, "Current user", (*handlers).profile},
		{http.MethodPut, "/api/rest/core/profile", Reader, "Update current user", (*handlers).updateProfile},
		{http.MethodGet, "/api/rest/core/project", Reader, "List REST and SOAP projects", (*handlers).projects},
		{http.MethodGet, "/api/rest/core/system", Admin, "System information", (*handlers).system},
		{http.MethodGet, "/api/rest/core/event", Reader, "List events", (*handlers).events},
		{http.MethodDelete, "/api/rest/core/event", Modifier, "Clear all events", (*handlers).clearEvents},
		{http.MethodPost, "/api/rest/core/expression/validate", Reader, "Render an expression body", (*handlers).validateExpression},
		{http.MethodGet, "/api/rest/core/user", Admin, "List users", (*handlers).users},
		{http.MethodPost, "/api/rest/core/user", Admin, "Create user", (*handlers).createUser},
		{http.MethodGet, "/api/rest/core/user/:userId", Admin, "Get user", (*handlers).user},
		{http.MethodPut, "/api/rest/core/user/:userId", Admin, "Update user", (*handlers).updateUser},
		{http.MethodDelete, "/api/rest/core/user/:userId", Admin, "Delete user", (*handlers).deleteUser},
		{http.MethodPost, "/api/rest/core/project/rest/import", Modifier, "Import REST project (deprecated alias)", (*handlers).importRestProject},
		{http.MethodPost, "/api/rest/core/project/soap/import", Modifier, "Import SOAP project (deprecated alias)", (*handlers).importSoapProject},

		// REST projects (§5.4).
		{http.MethodPost, "/api/rest/rest/project", Modifier, "Create REST project", (*handlers).createRestProject},
		{http.MethodPost, "/api/rest/rest/project/import", Modifier, "Import REST project (CastleMock XML or native JSON)", (*handlers).importRestProject},
		{http.MethodGet, restProject, Reader, "Get REST project", (*handlers).restProject},
		{http.MethodPut, restProject, Modifier, "Update REST project", (*handlers).updateRestProject},
		{http.MethodDelete, restProject, Modifier, "Delete REST project", (*handlers).deleteRestProject},
		{http.MethodPut, restProject + "/application/status", Modifier, "Set the status of every method of the applications", (*handlers).restApplicationsStatus},
		{http.MethodPut, restProject + "/application/endpoint/forwarded", Modifier, "Set the forwarded endpoint of every method of the applications", (*handlers).restApplicationsEndpoint},
		{http.MethodPut, restProject + "/port/endpoint/forwarded", Modifier, "Alias of .../application/endpoint/forwarded (D-11)", (*handlers).restApplicationsEndpoint},
		{http.MethodPost, restProject + "/definition/file", Modifier, "Import a REST definition file", (*handlers).restDefinitionFile},
		{http.MethodPost, restProject + "/definition/link", Modifier, "Import a REST definition from a URL", (*handlers).restDefinitionLink},
		{http.MethodGet, restProject + "/export", Reader, "Export REST project (?format=json for native JSON)", (*handlers).exportRestProject},
		{http.MethodPost, restProject + "/application", Modifier, "Create REST application", (*handlers).createRestApplication},
		{http.MethodGet, restApp, Reader, "Get REST application", (*handlers).restApplication},
		{http.MethodPut, restApp, Modifier, "Update REST application", (*handlers).updateRestApplication},
		{http.MethodDelete, restApp, Modifier, "Delete REST application", (*handlers).deleteRestApplication},
		{http.MethodPut, restApp + "/resource/status", Modifier, "Set the status of every method of the resources", (*handlers).restResourcesStatus},
		{http.MethodPut, restApp + "/resource/endpoint/forwarded", Modifier, "Set the forwarded endpoint of every method of the resources", (*handlers).restResourcesEndpoint},
		{http.MethodPost, restApp + "/resource", Modifier, "Create REST resource", (*handlers).createRestResource},
		{http.MethodGet, restResource, Reader, "Get REST resource", (*handlers).restResource},
		{http.MethodPut, restResource, Modifier, "Update REST resource", (*handlers).updateRestResource},
		{http.MethodDelete, restResource, Modifier, "Delete REST resource", (*handlers).deleteRestResource},
		{http.MethodGet, restResource + "/parameter", Reader, "Path parameters of the resource URI", (*handlers).restResourceParameters},
		{http.MethodPut, restResource + "/method/status", Modifier, "Set the status of methods", (*handlers).restMethodsStatus},
		{http.MethodPut, restResource + "/method/endpoint/forwarded", Modifier, "Set the forwarded endpoint of methods", (*handlers).restMethodsEndpoint},
		{http.MethodPost, restResource + "/method", Modifier, "Create REST method", (*handlers).createRestMethod},
		{http.MethodGet, restMethod, Reader, "Get REST method", (*handlers).restMethod},
		{http.MethodPut, restMethod, Modifier, "Update REST method", (*handlers).updateRestMethod},
		{http.MethodDelete, restMethod, Modifier, "Delete REST method", (*handlers).deleteRestMethod},
		{http.MethodPut, restMethod + "/mockresponse/status", Modifier, "Set the status of mock responses", (*handlers).restMockResponsesStatus},
		{http.MethodPost, restMethod + "/mockresponse", Modifier, "Create REST mock response", (*handlers).createRestMockResponse},
		{http.MethodPost, restMethod + "/mockresponse/duplicate", Modifier, "Duplicate REST mock responses", (*handlers).duplicateRestMockResponses},
		{http.MethodGet, restMock, Reader, "Get REST mock response", (*handlers).restMockResponse},
		{http.MethodPut, restMock, Modifier, "Update REST mock response", (*handlers).updateRestMockResponse},
		{http.MethodDelete, restMock, Modifier, "Delete REST mock response", (*handlers).deleteRestMockResponse},
		{http.MethodGet, "/api/rest/rest/event/:eventId", Reader, "Get REST event", (*handlers).restEvent},

		// SOAP projects (§5.5).
		{http.MethodPost, "/api/rest/soap/project", Modifier, "Create SOAP project", (*handlers).createSoapProject},
		{http.MethodPost, "/api/rest/soap/project/import", Modifier, "Import SOAP project (CastleMock XML or native JSON)", (*handlers).importSoapProject},
		{http.MethodGet, soapProject, Reader, "Get SOAP project", (*handlers).soapProject},
		{http.MethodPut, soapProject, Modifier, "Update SOAP project", (*handlers).updateSoapProject},
		{http.MethodDelete, soapProject, Modifier, "Delete SOAP project", (*handlers).deleteSoapProject},
		{http.MethodPut, soapProject + "/port/status", Modifier, "Set the status of every operation of the ports", (*handlers).soapPortsStatus},
		{http.MethodPut, soapProject + "/port/endpoint/forwarded", Modifier, "Set the forwarded endpoint of every operation of the ports", (*handlers).soapPortsEndpoint},
		{http.MethodPost, soapProject + "/wsdl/file", Modifier, "Import a WSDL file", (*handlers).wsdlFile},
		{http.MethodPost, soapProject + "/wsdl/link", Modifier, "Import a WSDL from a URL", (*handlers).wsdlLink},
		{http.MethodGet, soapProject + "/export", Reader, "Export SOAP project (?format=json for native JSON)", (*handlers).exportSoapProject},
		{http.MethodGet, soapPort, Reader, "Get SOAP port", (*handlers).soapPort},
		{http.MethodPut, soapPort, Modifier, "Update SOAP port URI", (*handlers).updateSoapPort},
		{http.MethodDelete, soapPort, Modifier, "Delete SOAP port", (*handlers).deleteSoapPort},
		{http.MethodPut, soapPort + "/operation/status", Modifier, "Set the status of operations", (*handlers).soapOperationsStatus},
		{http.MethodPut, soapPort + "/operation/endpoint/forwarded", Modifier, "Set the forwarded endpoint of operations", (*handlers).soapOperationsEndpoint},
		{http.MethodGet, soapOperation, Reader, "Get SOAP operation", (*handlers).soapOperation},
		{http.MethodPut, soapOperation, Modifier, "Update SOAP operation", (*handlers).updateSoapOperation},
		{http.MethodPut, soapOperation + "/mockresponse/status", Modifier, "Set the status of mock responses", (*handlers).soapMockResponsesStatus},
		{http.MethodPost, soapOperation + "/mockresponse", Modifier, "Create SOAP mock response", (*handlers).createSoapMockResponse},
		{http.MethodPost, soapOperation + "/mockresponse/duplicate", Modifier, "Duplicate SOAP mock responses", (*handlers).duplicateSoapMockResponses},
		{http.MethodGet, soapMock, Reader, "Get SOAP mock response", (*handlers).soapMockResponse},
		{http.MethodPut, soapMock, Modifier, "Update SOAP mock response", (*handlers).updateSoapMockResponse},
		{http.MethodDelete, soapMock, Modifier, "Delete SOAP mock response", (*handlers).deleteSoapMockResponse},
		{http.MethodGet, soapResource, Reader, "Get SOAP resource metadata", (*handlers).soapResource},
		{http.MethodGet, soapResource + "/content", Reader, "Get SOAP resource content", (*handlers).soapResourceContent},
		{http.MethodPost, soapResource + "/import", Modifier, "Upload a SOAP resource (WSDL replaces existing WSDLs)", (*handlers).importSoapResource},
		{http.MethodGet, "/api/rest/soap/event/:eventId", Reader, "Get SOAP event", (*handlers).soapEvent},
	}
}
