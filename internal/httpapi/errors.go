// Package httpapi implements the Go HTTP API: JSON command and query endpoints,
// stable error codes, request validation, transaction boundaries, the recovery
// startup flow and health checks. Error details are ordered uniformly by
// tunnel, chainage, cycle, rock zone, surface unit, spray layer, spray band and
// generation.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// ErrorResponse is the unified JSON failure body required by the public
// interface contract.
type ErrorResponse struct {
	ErrorCode      domain.ErrorCode `json:"error_code"`
	Message        string           `json:"message"`
	Retryable      bool             `json:"retryable"`
	OrderedReasons []domain.Reason  `json:"ordered_reasons,omitempty"`
	OperationID    string           `json:"operation_id,omitempty"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError converts a domain error to the unified JSON structure.
func writeError(w http.ResponseWriter, err error) {
	de := domain.AsError(err)
	status := http.StatusBadRequest
	switch de.Code {
	case domain.CodeNotFound:
		status = http.StatusNotFound
	case domain.CodeInternalError:
		status = http.StatusInternalServerError
	case domain.CodeLeaseConflict, domain.CodeLeaseExpired,
		domain.CodeGenerationConflict, domain.CodeIdempotencyConflict,
		domain.CodeTerminalAlreadySet:
		status = http.StatusConflict
	case domain.CodeDeviceRetryPending:
		status = http.StatusAccepted
	}
	writeJSON(w, status, ErrorResponse{
		ErrorCode:      de.Code,
		Message:        de.Message,
		Retryable:      de.Retryable,
		OrderedReasons: de.OrderedReasons,
		OperationID:    de.OperationID,
	})
}
