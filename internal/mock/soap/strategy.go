package soap

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"time"

	"github.com/duv0-x/mockingbird/internal/domain"
	"github.com/duv0-x/mockingbird/internal/mock/expression"
	"github.com/duv0-x/mockingbird/internal/mock/forward"
	"github.com/duv0-x/mockingbird/internal/mock/match"
)

const (
	forwardedResponseName = "Forwarded response"
	recordedResponseName  = "Recorded response"
	recordedDateLayout    = "2006/01/02 15:04:05"
	// errorCode is the forwarded status from which mockOnFailure applies.
	errorCode = http.StatusInternalServerError
)

// target identifies the operation being served.
type target struct {
	projectID string
	port      *domain.SoapPort
	operation *domain.SoapOperation
	url       string // request URL, for expressions
}

// process dispatches on the operation status (SoapStrategyFactory, stategy/*).
// A nil response without error is CastleMock's "Unable to process request".
func (e *Engine) process(ctx context.Context, req domain.SoapRequest, t target) (*domain.SoapResponse, error) {
	op := t.operation
	switch op.Status {
	case domain.SoapOperationDisabled:
		return nil, fmt.Errorf("%w: the requested SOAP operation, %s, is disabled", errNoResponse, op.Name)
	case domain.SoapOperationEcho:
		return &domain.SoapResponse{
			Body:             req.Body,
			ContentType:      req.ContentType,
			HTTPStatusCode:   http.StatusOK,
			HTTPHeaders:      []domain.HTTPHeader{{Name: "Content-Type", Value: req.ContentType}},
			ContentEncodings: []domain.ContentEncoding{},
		}, nil
	case domain.SoapOperationMocked:
		resp, err := e.mock(ctx, req, t)
		if err != nil || resp != nil {
			return resp, err
		}
		if domain.Deref(op.AutomaticForward) && op.ForwardedEndpoint != "" {
			return e.forward(ctx, req, op), nil
		}
		return nil, nil
	case domain.SoapOperationForwarded:
		resp := e.forward(ctx, req, op)
		if resp != nil && resp.HTTPStatusCode >= errorCode && domain.Deref(op.MockOnFailure) {
			return e.mock(ctx, req, t)
		}
		return resp, nil
	case domain.SoapOperationRecording, domain.SoapOperationRecordOnce:
		resp := e.forward(ctx, req, op)
		if resp == nil {
			return nil, nil
		}
		if resp.HTTPStatusCode >= errorCode && domain.Deref(op.MockOnFailure) {
			return e.mock(ctx, req, t)
		}
		e.record(t, *resp, op.Status == domain.SoapOperationRecordOnce)
		return resp, nil
	}
	return nil, fmt.Errorf("unable to map the following operation status to a response strategy: %s", op.Status)
}

// mock ports SoapMockStrategyResultFactory.getResponse.
func (e *Engine) mock(ctx context.Context, req domain.SoapRequest, t target) (*domain.SoapResponse, error) {
	op := t.operation
	var enabled []domain.SoapMockResponse
	for _, r := range op.MockResponses {
		if r.Status == domain.SoapMockResponseEnabled {
			enabled = append(enabled, r)
		}
	}
	slices.SortStableFunc(enabled, func(a, b domain.SoapMockResponse) int { return match.AlphanumCompare(a.Name, b.Name) })
	if len(enabled) == 0 {
		return nil, nil
	}
	var selected *domain.SoapMockResponse
	switch op.ResponseStrategy {
	case domain.SoapStrategyRandom:
		selected = &enabled[rand.IntN(len(enabled))]
	case domain.SoapStrategySequence:
		state := e.deps.Store.State()
		i, err := state.SequenceIndex(ctx, op.ID)
		if err != nil {
			return nil, fmt.Errorf("read sequence index of %s: %w", op.ID, err)
		}
		if i >= len(enabled) || i < 0 {
			i = 0
		}
		selected = &enabled[i]
		if err := state.SetSequenceIndex(ctx, op.ID, i+1); err != nil {
			return nil, fmt.Errorf("store sequence index of %s: %w", op.ID, err)
		}
	case domain.SoapStrategyXPathInput:
		for i := range enabled {
			if slices.ContainsFunc(enabled[i].XPathExpressions, func(x domain.SoapXPathExpression) bool {
				return match.XPathMatches(req.Envelope, x.Expression)
			}) {
				selected = &enabled[i]
				break
			}
		}
		if selected == nil && op.DefaultMockResponseID != "" {
			for i := range enabled {
				if enabled[i].ID == op.DefaultMockResponseID {
					selected = &enabled[i]
					break
				}
			}
		}
	}
	if selected == nil {
		return nil, nil
	}
	body := selected.Body
	if domain.Deref(selected.UsingExpressions) {
		reqBody, reqURL := req.Body, t.url
		body = expression.Render(body, expression.Input{Body: &reqBody, URL: &reqURL})
	}
	return &domain.SoapResponse{
		Body:             body,
		MockResponseName: selected.Name,
		HTTPStatusCode:   selected.HTTPStatusCode,
		HTTPHeaders:      append([]domain.HTTPHeader{}, selected.HTTPHeaders...),
		ContentEncodings: append([]domain.ContentEncoding{}, selected.ContentEncodings...),
	}, nil
}

// forward ports SoapClient.getResponse: the raw request goes to the
// forwarded endpoint as is (no path or query appended). A failure is logged
// and yields no response.
func (e *Engine) forward(ctx context.Context, req domain.SoapRequest, op *domain.SoapOperation) *domain.SoapResponse {
	if op.ForwardedEndpoint == "" || e.deps.Forward == nil {
		e.log.WarnContext(ctx, "unable to forward SOAP request: no forwarded endpoint", "operation", op.ID)
		return nil
	}
	body := req.Body
	resp, err := e.deps.Forward.Do(ctx, forward.Request{Method: req.HTTPMethod, URL: op.ForwardedEndpoint, Headers: req.HTTPHeaders, Body: &body})
	if err != nil {
		e.log.WarnContext(ctx, "unable to forward SOAP request", "operation", op.ID, "err", err)
		return nil
	}
	return &domain.SoapResponse{
		Body:             resp.Body,
		MockResponseName: forwardedResponseName,
		HTTPStatusCode:   resp.StatusCode,
		HTTPHeaders:      resp.Headers,
		ContentEncodings: resp.Encodings,
	}
}

// record stores a forwarded response as a new mock response in the
// background (SoapResponseConverter.toCreateSoapMockResponseInput); record
// once also switches the operation to MOCKED.
func (e *Engine) record(t target, resp domain.SoapResponse, recordOnce bool) {
	recorded := domain.SoapMockResponse{
		ID:               domain.NewID(),
		Name:             recordedResponseName + " " + time.Now().Format(recordedDateLayout),
		Body:             resp.Body,
		OperationID:      t.operation.ID,
		Status:           domain.SoapMockResponseEnabled,
		HTTPStatusCode:   resp.HTTPStatusCode,
		UsingExpressions: domain.Ptr(false),
		HTTPHeaders:      append([]domain.HTTPHeader{}, resp.HTTPHeaders...),
	}
	recorded.EnsureSlices()
	portID, opID := t.port.ID, t.operation.ID
	e.async(func(ctx context.Context) {
		_, err := e.deps.Store.Soap().Update(ctx, t.projectID, func(p *domain.SoapProject) error {
			port := p.Port(portID)
			if port == nil {
				return fmt.Errorf("port %s not found", portID)
			}
			op := port.Operation(opID)
			if op == nil {
				return fmt.Errorf("operation %s not found", opID)
			}
			op.MockResponses = append(op.MockResponses, recorded)
			if recordOnce {
				op.Status = domain.SoapOperationMocked
			}
			return nil
		})
		if err != nil {
			e.log.Error("unable to record SOAP response", "project", t.projectID, "operation", opID, "err", err)
		}
	})
}
