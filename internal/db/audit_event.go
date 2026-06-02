package db

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/audit"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/slicesx"

	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
)

type auditEventEntity struct {
	EID           string         `gorm:"column:id;type:uuid;default:uuid_generate_v4()"`
	ETimestamp    time.Time      `gorm:"column:timestamp;autoCreateTime:false"`
	ERealmID      *string        `gorm:"column:realm_id;type:uuid"`
	EActorID      string         `gorm:"column:actor_id"`
	EActorType    string         `gorm:"column:actor_type"`
	EResourceType string         `gorm:"column:resource_type"`
	EResourceID   string         `gorm:"column:resource_id"`
	EAction       string         `gorm:"column:action"`
	ESummary      string         `gorm:"column:summary"`
	EChanges      map[string]any `gorm:"column:changes;serializer:json"`
	EMetadata     map[string]any `gorm:"column:metadata;serializer:json"`
	EIP           string         `gorm:"column:ip"`
	ERequestID    string         `gorm:"column:request_id"`
}

func (auditEventEntity) TableName() string           { return "aegis.audit_event" }
func (e *auditEventEntity) ID() string               { return e.EID }
func (e *auditEventEntity) LID() string              { return "" }
func (e *auditEventEntity) Type() resource.Type      { return domain.ResourceTypeAuditEvent }
func (e *auditEventEntity) CreatedAt() time.Time     { return e.ETimestamp }
func (e *auditEventEntity) UpdatedAt() time.Time     { return e.ETimestamp }
func (e *auditEventEntity) DeletedAt() *time.Time    { return nil }
func (e *auditEventEntity) Timestamp() time.Time     { return e.ETimestamp }
func (e *auditEventEntity) ActorID() string          { return e.EActorID }
func (e *auditEventEntity) ActorType() string        { return e.EActorType }
func (e *auditEventEntity) ResourceType() string     { return e.EResourceType }
func (e *auditEventEntity) ResourceID() string       { return e.EResourceID }
func (e *auditEventEntity) Action() string           { return e.EAction }
func (e *auditEventEntity) Summary() string          { return e.ESummary }
func (e *auditEventEntity) Changes() map[string]any  { return e.EChanges }
func (e *auditEventEntity) Metadata() map[string]any { return e.EMetadata }
func (e *auditEventEntity) IP() string               { return e.EIP }
func (e *auditEventEntity) RequestID() string        { return e.ERequestID }
func (e *auditEventEntity) RealmID() string {
	if e.ERealmID == nil {
		return ""
	}
	return *e.ERealmID
}

var auditEventFieldMapping = map[string]string{
	fields.ID:           "id",
	fields.ActorID:      "actor_id",
	fields.Action:       "action",
	fields.ResourceType: "resource_type",
	fields.ResourceID:   "resource_id",
}

type auditEventReadRepo struct {
	*postgres.Repo
}

func NewAuditEventReadRepository(db *gormdb.DBClient) (*auditEventReadRepo, error) {
	r, err := postgres.NewRepo(db, auditEventFieldMapping)
	if err != nil {
		return nil, err
	}
	return &auditEventReadRepo{Repo: r}, nil
}

func (r *auditEventReadRepo) List(ctx context.Context, opts ...search.Option) (resource.ListResponse[domain.AuditEvent], error) {
	s := search.New(opts...)
	var found []*auditEventEntity
	if err := r.QueryApply(ctx, s.Query()).Order("timestamp DESC").Find(&found).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	var total int64
	if err := r.CountApply(ctx, new(auditEventEntity), s.Query()).Count(&total).Error; err != nil {
		return nil, postgres.NewErrUnknown(err)
	}
	out := slicesx.Map(found, func(e *auditEventEntity) domain.AuditEvent { return e })
	return resource.NewListResponse(out, int(total)), nil
}

// auditEventSink is the built-in Postgres AuditSink: it appends events to the
// partitioned, append-only aegis.audit_event table.
type auditEventSink struct {
	db *gormdb.DBClient
}

func NewAuditEventSink(db *gormdb.DBClient) *auditEventSink {
	return &auditEventSink{db: db}
}

func (s *auditEventSink) Emit(ctx context.Context, e audit.Event) error {
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	row := &auditEventEntity{
		ETimestamp:    ts,
		ERealmID:      nilIfEmpty(e.RealmID),
		EActorID:      e.ActorID,
		EActorType:    e.ActorType,
		EResourceType: e.ResourceType,
		EResourceID:   e.ResourceID,
		EAction:       e.Action,
		ESummary:      e.Summary,
		EChanges:      e.Changes,
		EMetadata:     e.Metadata,
		EIP:           e.IP,
		ERequestID:    e.RequestID,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
