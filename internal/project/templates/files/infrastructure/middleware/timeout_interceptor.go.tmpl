package middleware

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TimeoutInterceptor returns a gRPC unary server interceptor that enforces request timeouts.
// It ensures that slow or hanging handlers (e.g. database locks, third-party network stalls)
// cannot run indefinitely and leak server goroutines.
//
// Deadline Precedence Rules:
//  1. If the incoming request context already carries a client deadline tighter than effectiveTimeout,
//     the client's deadline is preserved and respected.
//  2. If the client did not set a deadline, or set a deadline longer than effectiveTimeout,
//     the server timeout is enforced using context.WithTimeout.
//  3. If methodTimeouts is provided, specific gRPC full methods (e.g. "/user.v1.UserService/ExportUsers")
//     can override the default timeout duration.
func TimeoutInterceptor(defaultTimeout time.Duration, methodTimeouts ...map[string]time.Duration) grpc.UnaryServerInterceptor {
	if defaultTimeout <= 0 {
		defaultTimeout = 15 * time.Second
	}

	var overrides map[string]time.Duration
	if len(methodTimeouts) > 0 && methodTimeouts[0] != nil {
		overrides = methodTimeouts[0]
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		timeout := defaultTimeout
		if overrides != nil {
			if custom, ok := overrides[info.FullMethod]; ok && custom > 0 {
				timeout = custom
			}
		}

		// Check if caller already provided an active deadline
		if clientDeadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(clientDeadline)
			// If client deadline is tighter or equal to server timeout, respect client deadline
			if remaining > 0 && remaining <= timeout {
				return handler(ctx, req)
			}
		}

		// Apply server-enforced timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		resp, err := handler(timeoutCtx, req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || timeoutCtx.Err() == context.DeadlineExceeded {
				return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
			}
			if errors.Is(err, context.Canceled) || timeoutCtx.Err() == context.Canceled {
				return nil, status.Error(codes.Canceled, "request canceled")
			}
			return nil, err
		}

		if timeoutCtx.Err() == context.DeadlineExceeded {
			return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
		}
		if timeoutCtx.Err() == context.Canceled {
			return nil, status.Error(codes.Canceled, "request canceled")
		}

		return resp, nil
	}
}
