package client

import (
	"context"
	"net/http"
	"time"

	"github.com/fromforgesoftware/aegis/internal/api"
)

// Role bundles permissions under a realm-scoped name.
type Role struct {
	ID           string
	RealmID      string
	Name         string
	ResourceType string
	Description  string
	Kind         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RoleComponent is one entry of a composed role: fold ComponentRoleID into
// the role's permission set with Operator (UNION|INTERSECT|EXCLUDE), applied
// in Ordinal order.
type RoleComponent struct {
	ComponentRoleID string
	Operator        string
	Ordinal         int
}

// RoleAPI manages roles (/api/roles), their permission sets and their
// composition. SetPermissions and SetComposition are idempotent overwrites.
type RoleAPI interface {
	Create(ctx context.Context, role Role, permissions []string) (Role, error)
	Get(ctx context.Context, id string) (Role, error)
	List(ctx context.Context, opts ...ListOption) ([]Role, error)
	Delete(ctx context.Context, id string) error

	Permissions(ctx context.Context, id string) ([]string, error)
	SetPermissions(ctx context.Context, id string, permissions []string) error
	Composition(ctx context.Context, id string) ([]RoleComponent, error)
	SetComposition(ctx context.Context, id string, components []RoleComponent) error
}

// ------------------------------------------------------------ HTTP

type roleHTTPClient struct {
	rest *restClient
}

func NewRoleHTTPClient(rest *restClient) *roleHTTPClient {
	return &roleHTTPClient{rest: rest}
}

// roleCreateDTO mirrors the server's create decode shape: the role body plus
// the synthetic "permissions" attribute seeding the initial permission set.
type roleCreateDTO struct {
	api.RoleDTO
	RPermissions []string `jsonapi:"attr,permissions,omitempty"`
}

func (c *roleHTTPClient) Create(ctx context.Context, role Role, permissions []string) (Role, error) {
	dto := &roleCreateDTO{RoleDTO: *roleToDTO(role), RPermissions: permissions}
	created, err := restCreate[*api.RoleDTO](ctx, c.rest, "/api/roles", dto)
	if err != nil {
		return Role{}, err
	}
	return roleFromDTO(created), nil
}

func (c *roleHTTPClient) Get(ctx context.Context, id string) (Role, error) {
	dto, err := restGet[*api.RoleDTO](ctx, c.rest, "/api/roles/"+id)
	if err != nil {
		return Role{}, err
	}
	return roleFromDTO(dto), nil
}

func (c *roleHTTPClient) List(ctx context.Context, opts ...ListOption) ([]Role, error) {
	dtos, err := restList[*api.RoleDTO](ctx, c.rest, "/api/roles", opts...)
	if err != nil {
		return nil, err
	}
	roles := make([]Role, 0, len(dtos))
	for _, dto := range dtos {
		roles = append(roles, roleFromDTO(dto))
	}
	return roles, nil
}

func (c *roleHTTPClient) Delete(ctx context.Context, id string) error {
	return restDelete(ctx, c.rest, "/api/roles/"+id)
}

func (c *roleHTTPClient) Permissions(ctx context.Context, id string) ([]string, error) {
	var doc identifierDocument
	if err := restJSON(ctx, c.rest, http.MethodGet, "/api/roles/"+id+"/permissions", nil, &doc); err != nil {
		return nil, err
	}
	return doc.ids(), nil
}

func (c *roleHTTPClient) SetPermissions(ctx context.Context, id string, permissions []string) error {
	doc := identifierDocumentOf(string(api.ResourceTypePermission), permissions)
	return restJSON(ctx, c.rest, http.MethodPost, "/api/roles/"+id+"/permissions", doc, nil)
}

// roleComponentDTO is the plain-JSON wire shape of one composition entry.
type roleComponentDTO struct {
	ComponentRoleID string `json:"componentRoleId"`
	Operator        string `json:"operator"`
	Ordinal         int    `json:"ordinal"`
}

type roleCompositionDocument struct {
	Data []roleComponentDTO `json:"data"`
}

func (c *roleHTTPClient) Composition(ctx context.Context, id string) ([]RoleComponent, error) {
	var doc roleCompositionDocument
	if err := restJSON(ctx, c.rest, http.MethodGet, "/api/roles/"+id+"/composition", nil, &doc); err != nil {
		return nil, err
	}
	components := make([]RoleComponent, 0, len(doc.Data))
	for _, component := range doc.Data {
		components = append(components, RoleComponent(component))
	}
	return components, nil
}

func (c *roleHTTPClient) SetComposition(ctx context.Context, id string, components []RoleComponent) error {
	doc := roleCompositionDocument{Data: make([]roleComponentDTO, 0, len(components))}
	for _, component := range components {
		doc.Data = append(doc.Data, roleComponentDTO(component))
	}
	return restJSON(ctx, c.rest, http.MethodPost, "/api/roles/"+id+"/composition", doc, nil)
}

func roleToDTO(role Role) *api.RoleDTO {
	dto := &api.RoleDTO{
		RRealmID:      role.RealmID,
		RName:         role.Name,
		RResourceType: role.ResourceType,
		RDescription:  role.Description,
		RKind:         role.Kind,
	}
	dto.RID = role.ID
	dto.RType = api.ResourceTypeRole
	return dto
}

func roleFromDTO(dto *api.RoleDTO) Role {
	if dto == nil {
		return Role{}
	}
	return Role{
		ID:           dto.ID(),
		RealmID:      dto.RRealmID,
		Name:         dto.RName,
		ResourceType: dto.RResourceType,
		Description:  dto.RDescription,
		Kind:         dto.RKind,
		CreatedAt:    dto.CreatedAt(),
		UpdatedAt:    dto.UpdatedAt(),
	}
}
