package db

import (
	"context"
	"encoding/json"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/postgres"

	"github.com/fromforgesoftware/aegis/internal/app"
)

// accountMergeRepo moves a source account's authz footprint onto a target and
// records the merge. Every method runs through WithContext, so when the
// usecase calls them inside a transactioner they all enlist in the same tx.
type accountMergeRepo struct {
	db *gormdb.DBClient
}

func NewAccountMergeRepository(db *gormdb.DBClient) *accountMergeRepo {
	return &accountMergeRepo{db: db}
}

// TransferExternalIDs reassigns the source's federated identities to the
// target. UNIQUE(kind, external_id) is global, so a given external id belongs
// to exactly one account and the move can never collide.
func (r *accountMergeRepo) TransferExternalIDs(ctx context.Context, source, target string) (int64, error) {
	tx := r.db.WithContext(ctx).Exec(
		"UPDATE aegis.account_external_id SET account_id = ? WHERE account_id = ?", target, source)
	if tx.Error != nil {
		return 0, postgres.NewErrUnknown(tx.Error)
	}
	return tx.RowsAffected, nil
}

// TransferMemberships moves the source into the target's groups, skipping sets
// the target already belongs to (PK is actor_set_id+account_id), then drops the
// source's leftover rows so no membership lingers on the merged-away account.
func (r *accountMergeRepo) TransferMemberships(ctx context.Context, source, target string) (int64, error) {
	moved := r.db.WithContext(ctx).Exec(
		`UPDATE aegis.actor_set_member SET account_id = ?
		 WHERE account_id = ?
		   AND actor_set_id NOT IN (SELECT actor_set_id FROM aegis.actor_set_member WHERE account_id = ?)`,
		target, source, target)
	if moved.Error != nil {
		return 0, postgres.NewErrUnknown(moved.Error)
	}
	if err := r.db.WithContext(ctx).Exec(
		"DELETE FROM aegis.actor_set_member WHERE account_id = ?", source).Error; err != nil {
		return 0, postgres.NewErrUnknown(err)
	}
	return moved.RowsAffected, nil
}

// TransferBindings reassigns the source's direct ACL bindings to the target,
// skipping (resource, role) pairs the target already holds — UNIQUE includes
// subject_id — then drops the source's leftovers.
func (r *accountMergeRepo) TransferBindings(ctx context.Context, source, target string) (int64, error) {
	moved := r.db.WithContext(ctx).Exec(
		`UPDATE aegis.acl SET subject_id = ?
		 WHERE subject_type = 'ACCOUNT' AND subject_id = ?
		   AND NOT EXISTS (
		       SELECT 1 FROM aegis.acl t
		       WHERE t.resource_id = aegis.acl.resource_id
		         AND t.role_id = aegis.acl.role_id
		         AND t.subject_type = 'ACCOUNT'
		         AND t.subject_id = ?)`,
		target, source, target)
	if moved.Error != nil {
		return 0, postgres.NewErrUnknown(moved.Error)
	}
	if err := r.db.WithContext(ctx).Exec(
		"DELETE FROM aegis.acl WHERE subject_type = 'ACCOUNT' AND subject_id = ?", source).Error; err != nil {
		return 0, postgres.NewErrUnknown(err)
	}
	return moved.RowsAffected, nil
}

// SoftDeleteSource disables and tombstones the merged-away account so it can no
// longer authenticate while staying traceable via the merge event.
func (r *accountMergeRepo) SoftDeleteSource(ctx context.Context, source string) error {
	if err := r.db.WithContext(ctx).Exec(
		"UPDATE aegis.account SET status = 'DISABLED', deleted_at = NOW(), updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
		source).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}

// RecordMergeEvent writes the audit trail row for the merge.
func (r *accountMergeRepo) RecordMergeEvent(ctx context.Context, e app.AccountMergeEvent) error {
	summary, err := json.Marshal(e.Summary)
	if err != nil {
		return postgres.NewErrUnknown(err)
	}
	if err := r.db.WithContext(ctx).Exec(
		"INSERT INTO aegis.account_merge_event (source_id, target_id, realm_id, summary) VALUES (?, ?, ?, ?)",
		e.SourceID, e.TargetID, e.RealmID, summary).Error; err != nil {
		return postgres.NewErrUnknown(err)
	}
	return nil
}
