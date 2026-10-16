// Package wsdl parses WSDL 1.1 documents into SOAP ports/operations and merges
// them into a SOAP project (SPEC.md §6.2).
//
// Port of service-mock-soap project/converter/** and CreateSoapPortsService.
// Consumer: internal/api (T4) for /wsdl/file and /wsdl/link.
//
// Owner: T3.
package wsdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/storage"
)

// Source is either uploaded bytes (Filename+Data) or a URL to download.
type Source struct {
	Filename string
	Data     []byte
	URL      string
}

// ErrInvalidWSDL wraps every failure caused by the document itself (not XML,
// missing binding/port type, ...), as opposed to download or storage errors.
var ErrInvalidWSDL = errors.New("unable to parse the WSDL file")

type Options struct {
	GenerateResponse bool
	// IncludeImports follows wsdl:import locations recursively (link import only).
	IncludeImports bool
	HTTPClient     *http.Client
}

// result is a parsed document: its ports, its resource metadata and content.
type result struct {
	document
	ports      []domain.SoapPort
	resourceID string
}

// Import converts src and merges it into the project (CreateSoapPortsService
// semantics), replacing the project's WSDL/WSDL_IMPORT resources.
func Import(ctx context.Context, repo storage.SoapRepository, projectID string, src Source, opts Options) error {
	documents, err := load(ctx, src, opts)
	if err != nil {
		return err
	}

	results := make([]result, 0, len(documents))
	for _, doc := range documents {
		ports, err := doc.soapPorts(projectID, opts.GenerateResponse)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrInvalidWSDL, doc.name, err)
		}
		for i := range ports {
			ports[i].EnsureSlices()
		}
		results = append(results, result{document: doc, ports: ports, resourceID: domain.NewID()})
	}

	if _, err := repo.Update(ctx, projectID, func(project *domain.SoapProject) error {
		for i := range results {
			mergePorts(project, results[i].ports)
		}
		// All existing WSDL and WSDL_IMPORT resources are replaced
		// (CreateSoapPortsService); SCHEMA resources are kept.
		kept := project.Resources[:0]
		for _, resource := range project.Resources {
			if resource.Type != domain.SoapResourceWSDL && resource.Type != domain.SoapResourceWSDLImport {
				kept = append(kept, resource)
			}
		}
		project.Resources = kept
		for i := range results {
			project.Resources = append(project.Resources, domain.SoapResource{
				ID:        results[i].resourceID,
				Name:      results[i].name,
				ProjectID: projectID,
				Type:      results[i].kind,
			})
		}
		return nil
	}); err != nil {
		return fmt.Errorf("wsdl: import into project %s: %w", projectID, err)
	}

	for i := range results {
		if err := repo.PutResourceContent(ctx, projectID, results[i].resourceID, results[i].raw); err != nil {
			return fmt.Errorf("wsdl: store resource %s of project %s: %w", results[i].resourceID, projectID, err)
		}
	}
	return nil
}

// mergePorts merges the parsed ports into the project (CreateSoapPortsService):
// an existing port of the same name keeps its operations; existing operations
// of the same name get originalEndpoint and soapVersion refreshed and new ones
// are appended with their responses. New ports are appended.
func mergePorts(project *domain.SoapProject, ports []domain.SoapPort) {
	for _, newPort := range ports {
		var existing *domain.SoapPort
		for i := range project.Ports {
			if project.Ports[i].Name == newPort.Name {
				existing = &project.Ports[i]
				break
			}
		}
		if existing == nil {
			project.Ports = append(project.Ports, newPort)
			continue
		}
		for _, newOperation := range newPort.Operations {
			var existingOperation *domain.SoapOperation
			for i := range existing.Operations {
				if existing.Operations[i].Name == newOperation.Name {
					existingOperation = &existing.Operations[i]
					break
				}
			}
			if existingOperation != nil {
				existingOperation.OriginalEndpoint = newOperation.OriginalEndpoint
				existingOperation.SoapVersion = newOperation.SoapVersion
				continue
			}
			newOperation.PortID = existing.ID
			existing.Operations = append(existing.Operations, newOperation)
		}
	}
}

// load parses the source documents: the uploaded file, or the downloaded link
// plus its followed imports.
func load(ctx context.Context, src Source, opts Options) ([]document, error) {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: time.Minute}
	}
	switch {
	case src.Data != nil:
		name := src.Filename
		if name == "" {
			name = "definition.wsdl"
		}
		doc, err := parseDocument(name, domain.SoapResourceWSDL, string(src.Data))
		if err != nil {
			return nil, err
		}
		return []document{doc}, nil
	case src.URL != "":
		raw, err := download(ctx, client, src.URL)
		if err != nil {
			return nil, err
		}
		doc, err := parseDocument(resourceName(src.URL), domain.SoapResourceWSDL, raw)
		if err != nil {
			return nil, err
		}
		documents := []document{doc}
		if opts.IncludeImports {
			documents, err = followImports(ctx, client, documents, src.URL)
			if err != nil {
				return nil, err
			}
		}
		return documents, nil
	default:
		return nil, errors.New("wsdl: neither files or links were provided when importing SOAP ports")
	}
}

// followImports ports SoapPortConverter.loadExternal: wsdl:import locations
// resolved relative to the importing URL, downloaded recursively and added as
// WSDL_IMPORT resources.
func followImports(ctx context.Context, client *http.Client, documents []document, mainURL string) ([]document, error) {
	visited := map[string]bool{mainURL: true}
	type item struct {
		docIndex int
		base     string
	}
	queue := []item{{0, mainURL}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, location := range documents[current.docIndex].importLocations() {
			importURL, err := resolveURL(current.base, location)
			if err != nil {
				continue // UrlUtility.getPath drops unresolvable references
			}
			if visited[importURL] {
				continue
			}
			visited[importURL] = true
			raw, err := download(ctx, client, importURL)
			if err != nil {
				return nil, err
			}
			doc, err := parseDocument(resourceName(importURL), domain.SoapResourceWSDLImport, raw)
			if err != nil {
				return nil, err
			}
			documents = append(documents, doc)
			queue = append(queue, item{len(documents) - 1, importURL})
		}
	}
	return documents, nil
}

// resolveURL ports UrlUtility.getPath: new URL(base, reference).
func resolveURL(base, reference string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	resolved, err := baseURL.Parse(reference)
	if err != nil {
		return "", err
	}
	return resolved.String(), nil
}

// download fetches the raw content of a WSDL link (FileManager.uploadFiles,
// whose url.openStream also fails on HTTP errors).
func download(ctx context.Context, client *http.Client, link string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", fmt.Errorf("wsdl: download %s: %w", link, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wsdl: download %s: %w", link, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("wsdl: download %s: status %d", link, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("wsdl: download %s: %w", link, err)
	}
	return string(body), nil
}

// resourceName is the last path segment of a link (mockingbird's documented
// replacement for CastleMock's UploadedFile-xxxx names).
func resourceName(link string) string {
	if u, err := url.Parse(link); err == nil {
		if i := strings.LastIndex(u.Path, "/"); i >= 0 && u.Path[i+1:] != "" {
			return u.Path[i+1:]
		}
	}
	return fmt.Sprintf("UploadedFile-%s.wsdl", domain.NewID())
}
