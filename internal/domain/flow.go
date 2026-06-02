package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeFlow is the JSON:API type for /api/auth/flows.
const ResourceTypeFlow resource.Type = "authFlows"

// FlowType is the kind of interactive auth flow a client is driving.
// FlowType() reports it; the embedded resource.Resource.Type() reports the
// JSON:API type, so the two never clash (same shape as Account).
type FlowType string

const (
	FlowTypeLogin        FlowType = "LOGIN"
	FlowTypeRegistration FlowType = "REGISTRATION"
	FlowTypeRecovery     FlowType = "RECOVERY"
	FlowTypeVerification FlowType = "VERIFICATION"
)

func (t FlowType) Valid() bool {
	switch t {
	case FlowTypeLogin, FlowTypeRegistration, FlowTypeRecovery, FlowTypeVerification:
		return true
	}
	return false
}

// FlowState is the lifecycle marker of a flow. A flow is PENDING until a
// submission completes it; expiry is derived from ExpiresAt, not a state.
type FlowState string

const (
	FlowStatePending   FlowState = "PENDING"
	FlowStateCompleted FlowState = "COMPLETED"
)

func (s FlowState) Valid() bool {
	switch s {
	case FlowStatePending, FlowStateCompleted:
		return true
	}
	return false
}

// FlowField describes one input a client must submit to advance a flow.
// Kind drives how a hosted page renders the control (email/password/text/token).
type FlowField struct {
	Name     string
	Kind     string
	Required bool
}

// RequiredFields returns the inputs a flow of the given type expects on
// submission — the contract a headless client or hosted page renders against.
func RequiredFields(t FlowType) []FlowField {
	switch t {
	case FlowTypeLogin:
		return []FlowField{{Name: "email", Kind: "email", Required: true}, {Name: "password", Kind: "password", Required: true}}
	case FlowTypeRegistration:
		return []FlowField{
			{Name: "email", Kind: "email", Required: true},
			{Name: "password", Kind: "password", Required: true},
			{Name: "displayName", Kind: "text", Required: false},
		}
	case FlowTypeRecovery:
		return []FlowField{{Name: "email", Kind: "email", Required: true}}
	case FlowTypeVerification:
		return []FlowField{{Name: "token", Kind: "token", Required: true}}
	}
	return nil
}

// Flow is a stateful interactive auth flow (login/registration/recovery/
// verification), persisted so it is resumable and drivable headless.
type Flow interface {
	resource.Resource
	RealmID() string
	FlowType() FlowType
	State() FlowState
	ExpiresAt() time.Time
	// ResultAccountID is the account a login/registration flow resolved to
	// on completion; empty for recovery/verification or while pending.
	ResultAccountID() string
}

type flow struct {
	resource.Resource

	realmID         string
	flowType        FlowType
	state           FlowState
	expiresAt       time.Time
	resultAccountID string
}

type FlowOption func(*flow)

func WithFlowID(id string) FlowOption {
	return func(f *flow) { f.Resource = resource.Update(f.Resource, resource.WithID(id)) }
}
func WithFlowState(s FlowState) FlowOption {
	return func(f *flow) { f.state = s }
}
func WithFlowResultAccountID(id string) FlowOption {
	return func(f *flow) { f.resultAccountID = id }
}

// NewFlow builds a flow aggregate (default state PENDING).
func NewFlow(realmID string, t FlowType, expiresAt time.Time, opts ...FlowOption) Flow {
	f := &flow{
		Resource:  resource.New(resource.WithType(ResourceTypeFlow)),
		realmID:   realmID,
		flowType:  t,
		state:     FlowStatePending,
		expiresAt: expiresAt,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *flow) RealmID() string         { return f.realmID }
func (f *flow) FlowType() FlowType      { return f.flowType }
func (f *flow) State() FlowState        { return f.state }
func (f *flow) ExpiresAt() time.Time    { return f.expiresAt }
func (f *flow) ResultAccountID() string { return f.resultAccountID }

// FlowExpired reports whether the flow can no longer be submitted at now.
func FlowExpired(f Flow, now time.Time) bool {
	return now.After(f.ExpiresAt())
}
