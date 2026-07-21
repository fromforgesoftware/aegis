package client

import (
	"context"
	"time"

	"github.com/fromforgesoftware/aegis/internal/api"
)

// Binding grants a subject (account or group) a role on a resource. A nil
// ExpiresAt means the grant does not expire. Writes only become visible to
// Check after the projection refreshes (AuthorizationAdminAPI.Refresh).
type Binding struct {
	ID          string
	ResourceID  string
	RoleID      string
	SubjectType string
	SubjectID   string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// BindingAPI manages ACL bindings (/api/bindings).
type BindingAPI interface {
	Create(ctx context.Context, binding Binding) (Binding, error)
	Get(ctx context.Context, id string) (Binding, error)
	List(ctx context.Context, opts ...ListOption) ([]Binding, error)
	Delete(ctx context.Context, id string) error
}

// ------------------------------------------------------------ HTTP

type bindingHTTPClient struct {
	rest *restClient
}

func NewBindingHTTPClient(rest *restClient) *bindingHTTPClient {
	return &bindingHTTPClient{rest: rest}
}

func (c *bindingHTTPClient) Create(ctx context.Context, binding Binding) (Binding, error) {
	dto, err := restCreate[*api.BindingDTO](ctx, c.rest, "/api/bindings", bindingToDTO(binding))
	if err != nil {
		return Binding{}, err
	}
	return bindingFromDTO(dto), nil
}

func (c *bindingHTTPClient) Get(ctx context.Context, id string) (Binding, error) {
	dto, err := restGet[*api.BindingDTO](ctx, c.rest, "/api/bindings/"+id)
	if err != nil {
		return Binding{}, err
	}
	return bindingFromDTO(dto), nil
}

func (c *bindingHTTPClient) List(ctx context.Context, opts ...ListOption) ([]Binding, error) {
	dtos, err := restList[*api.BindingDTO](ctx, c.rest, "/api/bindings", opts...)
	if err != nil {
		return nil, err
	}
	bindings := make([]Binding, 0, len(dtos))
	for _, dto := range dtos {
		bindings = append(bindings, bindingFromDTO(dto))
	}
	return bindings, nil
}

func (c *bindingHTTPClient) Delete(ctx context.Context, id string) error {
	return restDelete(ctx, c.rest, "/api/bindings/"+id)
}

func bindingToDTO(binding Binding) *api.BindingDTO {
	dto := &api.BindingDTO{
		RResourceID:  binding.ResourceID,
		RRoleID:      binding.RoleID,
		RSubjectType: binding.SubjectType,
		RSubjectID:   binding.SubjectID,
		RExpiresAt:   binding.ExpiresAt,
	}
	dto.RID = binding.ID
	dto.RType = api.ResourceTypeBinding
	return dto
}

func bindingFromDTO(dto *api.BindingDTO) Binding {
	if dto == nil {
		return Binding{}
	}
	return Binding{
		ID:          dto.ID(),
		ResourceID:  dto.RResourceID,
		RoleID:      dto.RRoleID,
		SubjectType: dto.RSubjectType,
		SubjectID:   dto.RSubjectID,
		ExpiresAt:   dto.RExpiresAt,
		CreatedAt:   dto.CreatedAt(),
		UpdatedAt:   dto.UpdatedAt(),
	}
}
