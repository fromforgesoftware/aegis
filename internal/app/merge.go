package app

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/persistence"
)

// MergeSummary counts what a merge moved from the source onto the target.
type MergeSummary struct {
	ExternalIDs int64 `json:"externalIds"`
	Memberships int64 `json:"memberships"`
	Bindings    int64 `json:"bindings"`
}

// AccountMergeEvent is the audit record persisted for a completed merge.
type AccountMergeEvent struct {
	SourceID string
	TargetID string
	RealmID  string
	Summary  MergeSummary
}

// AccountMergeRepository moves a source account's authz footprint onto a target
// and records the merge. Its methods run inside the usecase's transaction.
type AccountMergeRepository interface {
	TransferExternalIDs(ctx context.Context, source, target string) (int64, error)
	TransferMemberships(ctx context.Context, source, target string) (int64, error)
	TransferBindings(ctx context.Context, source, target string) (int64, error)
	SoftDeleteSource(ctx context.Context, source string) error
	RecordMergeEvent(ctx context.Context, e AccountMergeEvent) error
}

// AccountMergeUsecase consolidates a duplicate (source) account into a survivor
// (target): it reassigns the source's federated ids, group memberships, and
// direct bindings, tombstones the source, and records the merge — all in one
// transaction — then refreshes the authz projection so the moved grants take
// effect.
type AccountMergeUsecase interface {
	Merge(ctx context.Context, sourceID, targetID string) (MergeSummary, error)
}

type accountMergeUsecase struct {
	merge    AccountMergeRepository
	accounts AccountRepository
	authz    AuthorizationUsecase
	tx       persistence.Transactioner
	auditor  Auditor
}

func NewAccountMergeUsecase(
	merge AccountMergeRepository,
	accounts AccountRepository,
	authz AuthorizationUsecase,
	tx persistence.Transactioner,
	auditor Auditor,
) AccountMergeUsecase {
	return &accountMergeUsecase{merge: merge, accounts: accounts, authz: authz, tx: tx, auditor: auditor}
}

func (uc *accountMergeUsecase) Merge(ctx context.Context, sourceID, targetID string) (MergeSummary, error) {
	if sourceID == "" || targetID == "" {
		return MergeSummary{}, apierrors.InvalidArgument("source_id and target_id are required")
	}
	if sourceID == targetID {
		return MergeSummary{}, apierrors.InvalidArgument("cannot merge an account into itself")
	}

	source, err := uc.accounts.Get(ctx, byID(sourceID))
	if err != nil {
		return MergeSummary{}, mapAccountLookupErr(err, "source")
	}
	target, err := uc.accounts.Get(ctx, byID(targetID))
	if err != nil {
		return MergeSummary{}, mapAccountLookupErr(err, "target")
	}
	if source.RealmID() != target.RealmID() {
		return MergeSummary{}, apierrors.InvalidArgument("accounts belong to different realms")
	}

	var summary MergeSummary
	err = uc.tx.Exec(ctx, func(ctx context.Context) error {
		if summary.ExternalIDs, err = uc.merge.TransferExternalIDs(ctx, sourceID, targetID); err != nil {
			return err
		}
		if summary.Memberships, err = uc.merge.TransferMemberships(ctx, sourceID, targetID); err != nil {
			return err
		}
		if summary.Bindings, err = uc.merge.TransferBindings(ctx, sourceID, targetID); err != nil {
			return err
		}
		if err := uc.merge.SoftDeleteSource(ctx, sourceID); err != nil {
			return err
		}
		return uc.merge.RecordMergeEvent(ctx, AccountMergeEvent{
			SourceID: sourceID, TargetID: targetID, RealmID: target.RealmID(), Summary: summary,
		})
	})
	if err != nil {
		return MergeSummary{}, err
	}

	// Moved bindings only enter the closure after a projection refresh.
	if summary.Bindings > 0 {
		if err := uc.authz.Refresh(ctx); err != nil {
			return summary, err
		}
	}
	uc.auditor.Record(ctx, "account.merge", "account", targetID, map[string]any{
		"sourceId":    sourceID,
		"externalIds": summary.ExternalIDs,
		"memberships": summary.Memberships,
		"bindings":    summary.Bindings,
	})
	return summary, nil
}

func mapAccountLookupErr(err error, which string) error {
	if apierrors.Is(err, apierrors.CodeNotFound) {
		return apierrors.InvalidArgument(which + " account not found")
	}
	return err
}
