package domain

// OperationRecord is the persisted idempotency guard for every write command.
// It binds an operation id to a normalized request digest so that identical
// replays return the original result and conflicting content is rejected.
type OperationRecord struct {
	// OperationID is the idempotency key, unique within its scope.
	OperationID OperationID
	// Digest is the canonical JSON digest of the normalized request content.
	Digest string
	// Result summarizes the outcome for identical replays.
	Result string
	// ErrorCode is set when the original command failed.
	ErrorCode ErrorCode
	// LogicalTime records when the command was first applied.
	LogicalTime LogicalTime
}
