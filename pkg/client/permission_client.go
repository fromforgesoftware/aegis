package client

import (
	"context"
	"net/http"
	"time"

	"github.com/fromforgesoftware/aegis/internal/api"
)

// Permission is a catalog entry; the ID is the slug ("doc.read") composed of
// ResourceType and Verb, chosen by the caller at create time.
type Permission struct {
	ID           string
	ResourceType string
	Verb         string
	Description  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PermissionAPI manages the permission catalog (/api/permissions) and each
// permission's implication set (holding the permission also grants everything
// it implies, transitively). SetImplications is an idempotent overwrite.
type PermissionAPI interface {
	Create(ctx context.Context, permission Permission) (Permission, error)
	Get(ctx context.Context, id string) (Permission, error)
	List(ctx context.Context, opts ...ListOption) ([]Permission, error)
	Delete(ctx context.Context, id string) error

	Implications(ctx context.Context, id string) ([]string, error)
	SetImplications(ctx context.Context, id string, implied []string) error
}

// ------------------------------------------------------------ HTTP

type permissionHTTPClient struct {
	rest *restClient
}

func NewPermissionHTTPClient(rest *restClient) *permissionHTTPClient {
	return &permissionHTTPClient{rest: rest}
}

func (c *permissionHTTPClient) Create(ctx context.Context, permission Permission) (Permission, error) {
	dto, err := restCreate[*api.PermissionDTO](ctx, c.rest, "/api/permissions", permissionToDTO(permission))
	if err != nil {
		return Permission{}, err
	}
	return permissionFromDTO(dto), nil
}

func (c *permissionHTTPClient) Get(ctx context.Context, id string) (Permission, error) {
	dto, err := restGet[*api.PermissionDTO](ctx, c.rest, "/api/permissions/"+id)
	if err != nil {
		return Permission{}, err
	}
	return permissionFromDTO(dto), nil
}

func (c *permissionHTTPClient) List(ctx context.Context, opts ...ListOption) ([]Permission, error) {
	dtos, err := restList[*api.PermissionDTO](ctx, c.rest, "/api/permissions", opts...)
	if err != nil {
		return nil, err
	}
	permissions := make([]Permission, 0, len(dtos))
	for _, dto := range dtos {
		permissions = append(permissions, permissionFromDTO(dto))
	}
	return permissions, nil
}

func (c *permissionHTTPClient) Delete(ctx context.Context, id string) error {
	return restDelete(ctx, c.rest, "/api/permissions/"+id)
}

func (c *permissionHTTPClient) Implications(ctx context.Context, id string) ([]string, error) {
	var doc identifierDocument
	if err := restJSON(ctx, c.rest, http.MethodGet, "/api/permissions/"+id+"/implications", nil, &doc); err != nil {
		return nil, err
	}
	return doc.ids(), nil
}

func (c *permissionHTTPClient) SetImplications(ctx context.Context, id string, implied []string) error {
	doc := identifierDocumentOf(string(api.ResourceTypePermission), implied)
	return restJSON(ctx, c.rest, http.MethodPost, "/api/permissions/"+id+"/implications", doc, nil)
}

func permissionToDTO(permission Permission) *api.PermissionDTO {
	dto := &api.PermissionDTO{
		RResourceType: permission.ResourceType,
		RVerb:         permission.Verb,
		RDescription:  permission.Description,
	}
	dto.RID = permission.ID
	dto.RType = api.ResourceTypePermission
	return dto
}

func permissionFromDTO(dto *api.PermissionDTO) Permission {
	if dto == nil {
		return Permission{}
	}
	return Permission{
		ID:           dto.ID(),
		ResourceType: dto.RResourceType,
		Verb:         dto.RVerb,
		Description:  dto.RDescription,
		CreatedAt:    dto.CreatedAt(),
		UpdatedAt:    dto.UpdatedAt(),
	}
}
