package domain

// ErrorCode is a stable machine-readable error code shared across the HTTP API
// and the business packages. The codes are documented in the project
// specification and must not change.
type ErrorCode string

// Stable error codes required by the public interface contract.
const (
	CodeStaleSnapshot       ErrorCode = "STALE_SNAPSHOT"
	CodeInvalidGrid         ErrorCode = "INVALID_GRID"
	CodeNoSprayIntrusion    ErrorCode = "NO_SPRAY_INTRUSION"
	CodeBatchMismatch       ErrorCode = "BATCH_MISMATCH"
	CodeMassConflict        ErrorCode = "MASS_CONFLICT"
	CodeLeaseConflict       ErrorCode = "LEASE_CONFLICT"
	CodeLeaseExpired        ErrorCode = "LEASE_EXPIRED"
	CodeLayerOutOfOrder     ErrorCode = "LAYER_OUT_OF_ORDER"
	CodeBandDiscontinuity   ErrorCode = "BAND_DISCONTINUITY"
	CodeOverlapInsufficient ErrorCode = "OVERLAP_INSUFFICIENT"
	CodeFixedPointOverflow  ErrorCode = "FIXED_POINT_OVERFLOW"
	CodeDeviceRetryPending  ErrorCode = "DEVICE_RETRY_PENDING"
	CodeGenerationConflict  ErrorCode = "GENERATION_CONFLICT"
	CodeIdempotencyConflict ErrorCode = "IDEMPOTENCY_CONFLICT"
	CodeTerminalAlreadySet  ErrorCode = "TERMINAL_ALREADY_SET"
	CodeNotFound            ErrorCode = "NOT_FOUND"
	CodeInvalidRequest      ErrorCode = "INVALID_REQUEST"
	CodeInternalError       ErrorCode = "INTERNAL_ERROR"
)

// Reason is a single ordered diagnostic. OrderedReasons are always sorted by
// the canonical domain key (tunnel, chainage, cycle, rock zone, unit, layer,
// band, generation) before being returned.
type Reason struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Key     string    `json:"key,omitempty"`
}

// Error is the unified error structure returned by every failing API request.
// It carries the stable error code, a human message, whether the caller may
// retry, the ordered reasons and the operation id when one is available.
type Error struct {
	Code           ErrorCode `json:"error_code"`
	Message        string    `json:"message"`
	Retryable      bool      `json:"retryable"`
	OrderedReasons []Reason  `json:"ordered_reasons,omitempty"`
	OperationID    string    `json:"operation_id,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// NewError builds an Error with a single reason.
func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithOperation attaches an operation id for idempotency reporting.
func (e *Error) WithOperation(op string) *Error {
	e.OperationID = op
	return e
}

// WithReasons replaces the ordered reasons list.
func (e *Error) WithReasons(rs ...Reason) *Error {
	e.OrderedReasons = rs
	return e
}

// AsError converts an arbitrary error into *Error, wrapping unknown errors as
// internal errors.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	if de, ok := err.(*Error); ok {
		return de
	}
	return NewError(CodeInternalError, err.Error())
}
