package domain

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
)

// ResourceTypeAuditEvent is the JSON:API type for /api/audit-events.
const ResourceTypeAuditEvent resource.Type = "auditEvents"

// AuditEvent is a structured record of a state-changing operation.
type AuditEvent interface {
	resource.Resource
	Timestamp() time.Time
	ActorID() string
	ActorType() string
	ResourceType() string
	ResourceID() string
	Action() string
	Summary() string
	Changes() map[string]any
	Metadata() map[string]any
	RealmID() string
	IP() string
	RequestID() string
}

type auditEvent struct {
	resource.Resource

	timestamp    time.Time
	realmID      string
	actorID      string
	actorType    string
	resourceType string
	resourceID   string
	action       string
	summary      string
	changes      map[string]any
	metadata     map[string]any
	ip           string
	requestID    string
}

type AuditEventOption func(*auditEvent)

func WithAuditEventActor(id, actorType string) AuditEventOption {
	return func(e *auditEvent) { e.actorID = id; e.actorType = actorType }
}
func WithAuditEventRealmID(id string) AuditEventOption {
	return func(e *auditEvent) { e.realmID = id }
}
func WithAuditEventSummary(s string) AuditEventOption {
	return func(e *auditEvent) { e.summary = s }
}
func WithAuditEventChanges(c map[string]any) AuditEventOption {
	return func(e *auditEvent) { e.changes = c }
}
func WithAuditEventMetadata(m map[string]any) AuditEventOption {
	return func(e *auditEvent) { e.metadata = m }
}
func WithAuditEventTimestamp(t time.Time) AuditEventOption {
	return func(e *auditEvent) { e.timestamp = t }
}

// NewAuditEvent builds an event for an action on a resource. Action follows
// "{domain}.{verb}" (e.g. "binding.grant").
func NewAuditEvent(action, resourceType, resourceID string, opts ...AuditEventOption) AuditEvent {
	e := &auditEvent{
		Resource:     resource.New(resource.WithType(ResourceTypeAuditEvent)),
		action:       action,
		resourceType: resourceType,
		resourceID:   resourceID,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *auditEvent) Timestamp() time.Time     { return e.timestamp }
func (e *auditEvent) RealmID() string          { return e.realmID }
func (e *auditEvent) ActorID() string          { return e.actorID }
func (e *auditEvent) ActorType() string        { return e.actorType }
func (e *auditEvent) ResourceType() string     { return e.resourceType }
func (e *auditEvent) ResourceID() string       { return e.resourceID }
func (e *auditEvent) Action() string           { return e.action }
func (e *auditEvent) Summary() string          { return e.summary }
func (e *auditEvent) Changes() map[string]any  { return e.changes }
func (e *auditEvent) Metadata() map[string]any { return e.metadata }
func (e *auditEvent) IP() string               { return e.ip }
func (e *auditEvent) RequestID() string        { return e.requestID }
