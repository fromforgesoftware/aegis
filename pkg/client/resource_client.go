package client

import (
	"context"
	"time"

	"github.com/fromforgesoftware/aegis/internal/api"
)

// Subject types for a binding.
const (
	SubjectAccount = "ACCOUNT"
	SubjectGroup   = "ACTOR_SET"
)

// Visibility values (stored metadata on a resource; not consulted by Check).
const (
	VisibilityPrivate = "PRIVATE"
	VisibilityPublic  = "PUBLIC"
)

// Resource is an object registered for authorization. Aegis assigns the id;
// persist it next to the domain row it protects. A grant on ParentID cascades
// to this resource.
type Resource struct {
	ID             string
	RealmID        string
	Type           string
	OwnerAccountID string
	ParentID       string
	InheritVia     string
	Visibility     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ResourceAPI manages the authz resource registry (/api/resources).
type ResourceAPI interface {
	Create(ctx context.Context, resource Resource) (Resource, error)
	Get(ctx context.Context, id string) (Resource, error)
	List(ctx context.Context, opts ...ListOption) ([]Resource, error)
	Delete(ctx context.Context, id string) error
}

// ------------------------------------------------------------ HTTP

type resourceHTTPClient struct {
	rest *restClient
}

func NewResourceHTTPClient(rest *restClient) *resourceHTTPClient {
	return &resourceHTTPClient{rest: rest}
}

func (c *resourceHTTPClient) Create(ctx context.Context, resource Resource) (Resource, error) {
	dto, err := restCreate[*api.AuthzResourceDTO](ctx, c.rest, "/api/resources", resourceToDTO(resource))
	if err != nil {
		return Resource{}, err
	}
	return resourceFromDTO(dto), nil
}

func (c *resourceHTTPClient) Get(ctx context.Context, id string) (Resource, error) {
	dto, err := restGet[*api.AuthzResourceDTO](ctx, c.rest, "/api/resources/"+id)
	if err != nil {
		return Resource{}, err
	}
	return resourceFromDTO(dto), nil
}

func (c *resourceHTTPClient) List(ctx context.Context, opts ...ListOption) ([]Resource, error) {
	dtos, err := restList[*api.AuthzResourceDTO](ctx, c.rest, "/api/resources", opts...)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(dtos))
	for _, dto := range dtos {
		resources = append(resources, resourceFromDTO(dto))
	}
	return resources, nil
}

func (c *resourceHTTPClient) Delete(ctx context.Context, id string) error {
	return restDelete(ctx, c.rest, "/api/resources/"+id)
}

func resourceToDTO(resource Resource) *api.AuthzResourceDTO {
	dto := &api.AuthzResourceDTO{
		RRealmID:        resource.RealmID,
		RResourceType:   resource.Type,
		ROwnerAccountID: resource.OwnerAccountID,
		RParentID:       resource.ParentID,
		RInheritVia:     resource.InheritVia,
		RVisibility:     resource.Visibility,
	}
	dto.RID = resource.ID
	dto.RType = api.ResourceTypeAuthzResource
	return dto
}

func resourceFromDTO(dto *api.AuthzResourceDTO) Resource {
	if dto == nil {
		return Resource{}
	}
	return Resource{
		ID:             dto.ID(),
		RealmID:        dto.RRealmID,
		Type:           dto.RResourceType,
		OwnerAccountID: dto.ROwnerAccountID,
		ParentID:       dto.RParentID,
		InheritVia:     dto.RInheritVia,
		Visibility:     dto.RVisibility,
		CreatedAt:      dto.CreatedAt(),
		UpdatedAt:      dto.UpdatedAt(),
	}
}
