package fetcher

import (
	"context"
	"errors"
	"net"
)

func classifyFetchErrReason(err error) string {
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return "dns_error"
	}

	if netErr, ok := errors.AsType[net.Error](err); ok {
		if netErr.Timeout() || errors.Is(err, context.DeadlineExceeded) {
			return "timeout"
		}
		return "connection_error"
	}

	return "parse_error"
}
