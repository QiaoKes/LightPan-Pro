package driver

import (
	"context"
	"litepan/internal/domain"
)

// CapabilityAuthProvider reports independently verified credential capabilities.
type CapabilityAuthProvider interface {
	CredentialHealth() map[domain.AuthCapability]domain.CredentialHealth
}

// CapabilityAuthError must never trigger whole-account credential refresh.
type CapabilityAuthError struct {
	Capability domain.AuthCapability
	Cause      error
}

func (e *CapabilityAuthError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "credential capability unavailable: " + string(e.Capability)
}
func (e *CapabilityAuthError) Unwrap() error { return e.Cause }

// CredentialUpdateValidator validates changed credentials against retained verified identity.
type CredentialUpdateValidator interface {
	ValidateCredentialUpdate(context.Context, domain.AuthCredentials) error
}
