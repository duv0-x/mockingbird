package soap

import (
	"fmt"
	"strings"

	"github.com/duv0-x/mockingbird/internal/domain"
)

// identify ports IdentifySoapOperationService: the port whose URI equals the
// request URI, then SoapOperationFileRepository.findWithMethodAndVersionAndIdentifier.
func identify(project *domain.SoapProject, req domain.SoapRequest) (*domain.SoapPort, *domain.SoapOperation, error) {
	var port *domain.SoapPort
	for i := range project.Ports {
		if project.Ports[i].URI == req.URI {
			port = &project.Ports[i]
			break
		}
	}
	if port == nil {
		return nil, nil, fmt.Errorf("unable to find port with the following URI: %s", req.URI)
	}
	for i := range port.Operations {
		op := &port.Operations[i]
		if op.HTTPMethod != req.HTTPMethod || op.SoapVersion != req.SoapVersion ||
			!strings.EqualFold(op.OperationIdentifier.Name, req.OperationIdentifier.Name) {
			continue
		}
		// An operation without namespace (imported before namespaces were
		// stored) or identified by ELEMENT matches on the name alone.
		if op.OperationIdentifier.Namespace == "" || op.IdentifyStrategy == domain.IdentifyByElement ||
			strings.EqualFold(op.OperationIdentifier.Namespace, req.OperationIdentifier.Namespace) {
			return port, op, nil
		}
	}
	return nil, nil, fmt.Errorf("unable to identify SOAP operation: %s", req.URI)
}
