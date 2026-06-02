package internaltest

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// flow is a test stub implementing domain.Flow, used for fixtures and
// mock matchers without depending on the GORM-backed entity in db/.
type flow struct {
	id              string
	createdAt       time.Time
	updatedAt       time.Time
	deletedAt       *time.Time
	realmID         string
	flowType        domain.FlowType
	state           domain.FlowState
	expiresAt       time.Time
	resultAccountID string
}

type FlowOption func(*flow)

func defaultFlowOptions() []FlowOption {
	return []FlowOption{
		WithFlowID("flow-test"),
		WithFlowRealmID("realm-test"),
		WithFlowType(domain.FlowTypeLogin),
		WithFlowState(domain.FlowStatePending),
		WithFlowExpiresAt(time.Now().Add(30 * time.Minute)),
	}
}

func WithFlowID(id string) FlowOption {
	return func(f *flow) { f.id = id }
}
func WithFlowRealmID(realmID string) FlowOption {
	return func(f *flow) { f.realmID = realmID }
}
func WithFlowType(t domain.FlowType) FlowOption {
	return func(f *flow) { f.flowType = t }
}
func WithFlowState(s domain.FlowState) FlowOption {
	return func(f *flow) { f.state = s }
}
func WithFlowExpiresAt(t time.Time) FlowOption {
	return func(f *flow) { f.expiresAt = t }
}
func WithFlowResultAccountID(id string) FlowOption {
	return func(f *flow) { f.resultAccountID = id }
}

// NewFlow builds a domain.Flow fixture from the defaults overridden by opts.
func NewFlow(opts ...FlowOption) domain.Flow {
	f := &flow{}
	for _, opt := range append(defaultFlowOptions(), opts...) {
		opt(f)
	}
	return f
}

func (f *flow) ID() string                { return f.id }
func (f *flow) LID() string               { return "" }
func (f *flow) Type() resource.Type       { return domain.ResourceTypeFlow }
func (f *flow) CreatedAt() time.Time      { return f.createdAt }
func (f *flow) UpdatedAt() time.Time      { return f.updatedAt }
func (f *flow) DeletedAt() *time.Time     { return f.deletedAt }
func (f *flow) RealmID() string           { return f.realmID }
func (f *flow) FlowType() domain.FlowType { return f.flowType }
func (f *flow) State() domain.FlowState   { return f.state }
func (f *flow) ExpiresAt() time.Time      { return f.expiresAt }
func (f *flow) ResultAccountID() string   { return f.resultAccountID }

// MatchFlow compares the identifying fields (realm, type, state), ignoring
// server-volatile id/timestamps. Use inside mock.MatchedBy for Create args.
func MatchFlow(want domain.Flow) func(domain.Flow) bool {
	return func(got domain.Flow) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.RealmID() == got.RealmID() &&
			want.FlowType() == got.FlowType() &&
			want.State() == got.State()
	}
}
