package evidence

import "github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"

// Backoff returns the deterministic retry deadline offset for an attempt
// number. The first retry (attempt becomes 1) waits one base interval, then the
// wait doubles per subsequent attempt and is capped so a stuck instrument cannot
// overflow the logical clock. Because the offset depends only on the attempt
// number and the base, recovery from disk reproduces the exact same sequence.
func Backoff(attempt int64, base domain.LogicalTime) domain.LogicalTime {
	if base < 0 {
		base = 0
	}
	if attempt <= 0 {
		return base
	}
	shift := attempt - 1
	if shift > 20 {
		shift = 20
	}
	offset := base << shift
	const cap = domain.LogicalTime(1) << 40
	if offset > cap {
		offset = cap
	}
	return offset
}

// ClassifyFault maps a scripted instrument failure description to a stable
// fault type. Unknown descriptions are treated as a rejection so that no
// default reading is ever fabricated from an unrecognized outcome.
func ClassifyFault(description string) FaultType {
	switch description {
	case "timeout":
		return FaultTimeout
	case "disconnect":
		return FaultDisconnect
	case "bad_format":
		return FaultBadFormat
	default:
		return FaultRejected
	}
}

// Receipt carries a scripted instrument result: either a valid reading or a
// stable fault. It also carries the fence token and expected attempt so that
// stale or out-of-order receipts can be filtered.
type Receipt struct {
	Fence   string    `json:"fence_token"`
	Attempt int64     `json:"attempt"`
	Fault   FaultType `json:"fault,omitempty"`
	Value   string    `json:"value,omitempty"`
}

// SubmitReceipt applies a receipt to a device call. It rejects a foreign fence
// token and an attempt number that does not match the call's current attempt,
// ensuring stale or reordered receipts never advance spray, cure or detection
// prefixes. A fault advances the call to a retry state; a value marks success.
func (c *DeviceCall) SubmitReceipt(r Receipt, now domain.LogicalTime, base domain.LogicalTime) *domain.Error {
	if c.FenceToken != r.Fence {
		return domain.NewError(domain.CodeGenerationConflict, "stale fence token")
	}
	if r.Attempt != c.Attempt {
		return domain.NewError(domain.CodeGenerationConflict, "out-of-order receipt")
	}
	if r.Fault != "" {
		c.Fault = r.Fault
		c.Retry(now, Backoff(c.Attempt+1, base))
		return nil
	}
	if r.Value == "" {
		return domain.NewError(domain.CodeInvalidRequest, "receipt value required")
	}
	c.Succeed(r.Value)
	return nil
}

// Pending reports whether the call has not yet produced a valid reading.
func (c DeviceCall) Pending() bool {
	return c.Status == CallPending || c.Status == CallRetrying
}
