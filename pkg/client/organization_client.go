package client

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"

	"github.com/fromforgesoftware/aegis/internal/api"
)

// Organization is a tenant (an org/workspace) as seen by consumer services.
// OwnerID is the owning account; RealmID the realm it lives in.
type Organization struct {
	ID        string
	RealmID   string
	OwnerID   string
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrganizationAPI reads tenants (/api/organizations) over the gateway-gated
// admin surface. Self-service creation/activation stays with the realm user
// (the SPA); this API is for services that need to resolve an org and its
// owner — e.g. dev tooling locating a bootstrap-seeded workspace.
type OrganizationAPI interface {
	Get(ctx context.Context, id string) (Organization, error)
	// GetBySlug resolves an org by its unique slug; NotFound when absent and
	// Conflict when the slug is ambiguous across realms.
	GetBySlug(ctx context.Context, slug string) (Organization, error)
	List(ctx context.Context, opts ...ListOption) ([]Organization, error)
}

// ------------------------------------------------------------ HTTP

type organizationHTTPClient struct {
	rest *restClient
}

func NewOrganizationHTTPClient(rest *restClient) *organizationHTTPClient {
	return &organizationHTTPClient{rest: rest}
}

func (c *organizationHTTPClient) Get(ctx context.Context, id string) (Organization, error) {
	dto, err := restGet[*api.OrganizationDTO](ctx, c.rest, "/api/organizations/"+id)
	if err != nil {
		return Organization{}, err
	}
	return organizationFromDTO(dto), nil
}

func (c *organizationHTTPClient) GetBySlug(ctx context.Context, slug string) (Organization, error) {
	orgs, err := c.List(ctx, WithFilter("slug", "eq", slug))
	if err != nil {
		return Organization{}, err
	}
	switch len(orgs) {
	case 0:
		return Organization{}, apierrors.NotFound("organization", slug)
	case 1:
		return orgs[0], nil
	default:
		return Organization{}, apierrors.New(apierrors.CodeConflict,
			apierrors.WithMessage("organization slug \""+slug+"\" is ambiguous across realms"))
	}
}

func (c *organizationHTTPClient) List(ctx context.Context, opts ...ListOption) ([]Organization, error) {
	dtos, err := restList[*api.OrganizationDTO](ctx, c.rest, "/api/organizations", opts...)
	if err != nil {
		return nil, err
	}
	orgs := make([]Organization, 0, len(dtos))
	for _, dto := range dtos {
		orgs = append(orgs, organizationFromDTO(dto))
	}
	return orgs, nil
}

func organizationFromDTO(dto *api.OrganizationDTO) Organization {
	if dto == nil {
		return Organization{}
	}
	out := Organization{
		ID:        dto.ID(),
		RealmID:   dto.RRealmID,
		Name:      dto.RName,
		Slug:      dto.RSlug,
		CreatedAt: dto.CreatedAt(),
		UpdatedAt: dto.UpdatedAt(),
	}
	if dto.RRealm != nil {
		out.RealmID = dto.RRealm.ID()
	}
	if dto.ROwner != nil {
		out.OwnerID = dto.ROwner.ID()
	}
	return out
}
