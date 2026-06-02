package app

import (
	"context"
	"sync"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// QuotaPolicyRepository persists per-realm quota caps.
type QuotaPolicyRepository interface {
	repository.Creator[domain.QuotaPolicy]
	repository.Getter[domain.QuotaPolicy]
	repository.Lister[domain.QuotaPolicy]
	repository.Deleter
	GetByRealmResourceType(ctx context.Context, realmID, resourceType string) (domain.QuotaPolicy, error)
}

// UsageCounter reports how many of a resource a realm currently holds. The
// consuming service registers one per resource_type — the pluggable check that
// keeps Aegis ignorant of what it's counting.
type UsageCounter interface {
	Count(ctx context.Context, realmID string) (int, error)
}

// QuotaUsecase manages quota policies and enforces them. Allow checks a caller-
// supplied count; Check uses a registered UsageCounter.
type QuotaUsecase interface {
	repository.Getter[domain.QuotaPolicy]
	repository.Lister[domain.QuotaPolicy]
	repository.Deleter
	SetPolicy(ctx context.Context, p domain.QuotaPolicy) (domain.QuotaPolicy, error)
	Allow(ctx context.Context, realmID, resourceType string, current int) (bool, error)
	Register(resourceType string, counter UsageCounter)
	Check(ctx context.Context, realmID, resourceType string) (bool, error)
}

type quotaUsecase struct {
	usecase.Getter[domain.QuotaPolicy]
	usecase.Lister[domain.QuotaPolicy]
	repository.Deleter

	policies QuotaPolicyRepository

	mu       sync.RWMutex
	counters map[string]UsageCounter
}

func NewQuotaUsecase(policies QuotaPolicyRepository) QuotaUsecase {
	return &quotaUsecase{
		Getter:   usecase.NewGetter(policies, domain.ResourceTypeQuotaPolicy),
		Lister:   usecase.NewLister(policies),
		Deleter:  usecase.NewDeleter(policies),
		policies: policies,
		counters: map[string]UsageCounter{},
	}
}

func (uc *quotaUsecase) SetPolicy(ctx context.Context, p domain.QuotaPolicy) (domain.QuotaPolicy, error) {
	if p.RealmID() == "" || p.ResourceType() == "" {
		return nil, apierrors.InvalidArgument("realm_id and resource_type are required")
	}
	if p.MaxCount() < 0 {
		return nil, apierrors.InvalidArgument("max_count must not be negative")
	}
	return uc.policies.Create(ctx, p)
}

// Allow reports whether current usage leaves room under the realm's cap. A
// realm with no policy for the type is unconstrained.
func (uc *quotaUsecase) Allow(ctx context.Context, realmID, resourceType string, current int) (bool, error) {
	policy, err := uc.policies.GetByRealmResourceType(ctx, realmID, resourceType)
	if err != nil {
		if apierrors.Is(err, apierrors.CodeNotFound) {
			return true, nil
		}
		return false, err
	}
	return current < policy.MaxCount(), nil
}

func (uc *quotaUsecase) Register(resourceType string, counter UsageCounter) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.counters[resourceType] = counter
}

// Check enforces the quota using the registered counter for the resource type.
func (uc *quotaUsecase) Check(ctx context.Context, realmID, resourceType string) (bool, error) {
	uc.mu.RLock()
	counter := uc.counters[resourceType]
	uc.mu.RUnlock()
	if counter == nil {
		return false, apierrors.InvalidArgument("no usage counter registered for resource_type")
	}
	current, err := counter.Count(ctx, realmID)
	if err != nil {
		return false, err
	}
	return uc.Allow(ctx, realmID, resourceType, current)
}
