// Package protocol defines the transports currently supported by GoStarter.
package protocol

import (
	"fmt"
	"strings"
)

// Normalize returns a canonical protocol, defaulting an empty value to dual.
func Normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "dual"
	}
	return value
}

// Validate rejects transports that the current upstream profile cannot generate.
func Validate(value string) error {
	switch Normalize(value) {
	case "dual", "grpc":
		return nil
	case "rest":
		return fmt.Errorf("REST-only is not supported by the current upstream; use dual or grpc")
	default:
		return fmt.Errorf("unsupported protocol %q; use dual or grpc", value)
	}
}
