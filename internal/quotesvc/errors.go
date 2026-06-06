package quotesvc

import (
	"context"
	"errors"
	"net"

	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

// FailureKind classifies why a batch attempt failed, so recoverable transport
// problems can be retried while decode (parser) failures are preserved.
type FailureKind string

const (
	FailureNone         FailureKind = ""
	FailureTransport    FailureKind = "transport"
	FailureTimeout      FailureKind = "timeout"
	FailureServerSelect FailureKind = "server_select"
	FailureRateLimit    FailureKind = "rate_limit"
	FailureDecode       FailureKind = "decode"
	FailureCancelled    FailureKind = "cancelled"
)

// errServerSelect marks a failure to obtain any usable server connection.
var errServerSelect = errors.New("no usable TDX HQ server")

// classifyFailure maps an error to a FailureKind. Order matters: decode and
// cancellation are checked before generic transport classification.
func classifyFailure(err error) FailureKind {
	if err == nil {
		return FailureNone
	}
	switch {
	case tdx.IsQuoteDecodeError(err):
		return FailureDecode
	case errors.Is(err, context.Canceled):
		return FailureCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return FailureTimeout
	case errors.Is(err, errServerSelect), errors.Is(err, ErrPoolExhausted), errors.Is(err, ErrPoolClosed):
		return FailureServerSelect
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return FailureTimeout
	}
	return FailureTransport
}

// recoverable reports whether a failure kind is worth retrying. Decode failures
// are not retried so parser regressions surface clearly; cancellation is not a
// failure to retry.
func recoverable(kind FailureKind) bool {
	switch kind {
	case FailureTransport, FailureTimeout, FailureServerSelect, FailureRateLimit:
		return true
	default:
		return false
	}
}

func isCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
