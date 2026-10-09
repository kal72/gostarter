package protocol_test

import (
	"testing"

	"gostarter/internal/project/protocol"
)

func TestProtocolValidation(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"", "dual"}, {" dual ", "dual"}, {"GRPC", "grpc"}} {
		if err := protocol.Validate(tc.input); err != nil || protocol.Normalize(tc.input) != tc.want {
			t.Errorf("%q: normalize/validate failed: %v", tc.input, err)
		}
	}
	for _, value := range []string{"rest", " REST ", "http", "unknown"} {
		if protocol.Validate(value) == nil {
			t.Errorf("accepted unsupported protocol %q", value)
		}
	}
}
