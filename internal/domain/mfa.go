package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// MFAFactor is the kind of second factor.
type MFAFactor string

const (
	MFAFactorTOTP     MFAFactor = "TOTP"
	MFAFactorRecovery MFAFactor = "RECOVERY"
	MFAFactorWebAuthn MFAFactor = "WEBAUTHN"
)

func (f MFAFactor) Valid() bool {
	switch f {
	case MFAFactorTOTP, MFAFactorRecovery, MFAFactorWebAuthn:
		return true
	}
	return false
}

const (
	ResourceTypeMFAEnrollment  resource.Type = "mfaEnrollments"
	ResourceTypeRealmACRPolicy resource.Type = "realmAcrPolicies"
)

// MFAEnrollment is a per-account factor; for TOTP it carries the sealed shared
// secret and a confirmation timestamp set on first successful verify.
type MFAEnrollment interface {
	resource.Resource
	AccountID() string
	Factor() MFAFactor
	Secret() string
	ConfirmedAt() *time.Time
}

type mfaEnrollment struct {
	resource.Resource

	accountID   string
	factor      MFAFactor
	secret      string
	confirmedAt *time.Time
}

type MFAEnrollmentOption func(*mfaEnrollment)

func WithMFAEnrollmentID(id string) MFAEnrollmentOption {
	return func(e *mfaEnrollment) { e.Resource = resource.Update(e.Resource, resource.WithID(id)) }
}
func WithMFAEnrollmentSecret(s string) MFAEnrollmentOption {
	return func(e *mfaEnrollment) { e.secret = s }
}
func WithMFAEnrollmentConfirmedAt(t *time.Time) MFAEnrollmentOption {
	return func(e *mfaEnrollment) { e.confirmedAt = t }
}

func NewMFAEnrollment(accountID string, factor MFAFactor, opts ...MFAEnrollmentOption) MFAEnrollment {
	e := &mfaEnrollment{
		Resource:  resource.New(resource.WithType(ResourceTypeMFAEnrollment)),
		accountID: accountID,
		factor:    factor,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *mfaEnrollment) AccountID() string       { return e.accountID }
func (e *mfaEnrollment) Factor() MFAFactor       { return e.factor }
func (e *mfaEnrollment) Secret() string          { return e.secret }
func (e *mfaEnrollment) ConfirmedAt() *time.Time { return e.confirmedAt }

// StepUpToken proves a fresh second-factor re-auth, valid until ExpiresAt, for
// elevating a request's assurance (ACR) before a sensitive operation.
type StepUpToken interface {
	resource.Resource
	AccountID() string
	Factor() MFAFactor
	ACR() string
	ExpiresAt() time.Time
}

type stepUpToken struct {
	resource.Resource

	accountID string
	factor    MFAFactor
	acr       string
	expiresAt time.Time
}

func NewStepUpToken(id, accountID string, factor MFAFactor, acr string, expiresAt time.Time) StepUpToken {
	return &stepUpToken{
		Resource:  resource.New(resource.WithID(id)),
		accountID: accountID,
		factor:    factor,
		acr:       acr,
		expiresAt: expiresAt,
	}
}

func (t *stepUpToken) AccountID() string    { return t.accountID }
func (t *stepUpToken) Factor() MFAFactor    { return t.factor }
func (t *stepUpToken) ACR() string          { return t.acr }
func (t *stepUpToken) ExpiresAt() time.Time { return t.expiresAt }

// RealmACRPolicy declares whether a realm requires MFA and the assurance level
// a step-up confers.
type RealmACRPolicy interface {
	resource.Resource
	RealmID() string
	MFARequired() bool
	RequiredACR() string
}

type realmACRPolicy struct {
	resource.Resource

	realmID     string
	mfaRequired bool
	requiredACR string
}

type RealmACRPolicyOption func(*realmACRPolicy)

func WithRealmACRPolicyID(id string) RealmACRPolicyOption {
	return func(p *realmACRPolicy) { p.Resource = resource.Update(p.Resource, resource.WithID(id)) }
}

func NewRealmACRPolicy(realmID string, mfaRequired bool, requiredACR string, opts ...RealmACRPolicyOption) RealmACRPolicy {
	if requiredACR == "" {
		requiredACR = "aal2"
	}
	p := &realmACRPolicy{
		Resource:    resource.New(resource.WithType(ResourceTypeRealmACRPolicy)),
		realmID:     realmID,
		mfaRequired: mfaRequired,
		requiredACR: requiredACR,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *realmACRPolicy) RealmID() string     { return p.realmID }
func (p *realmACRPolicy) MFARequired() bool   { return p.mfaRequired }
func (p *realmACRPolicy) RequiredACR() string { return p.requiredACR }
