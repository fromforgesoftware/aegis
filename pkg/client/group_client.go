package client

import (
	"context"
	"net/http"
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/api"
)

// Group is a set of accounts usable as a binding subject; members inherit
// every role bound to the group. OrganizationID scopes the group to an
// organization ("team"); empty means realm-level.
type Group struct {
	ID             string
	RealmID        string
	Name           string
	Description    string
	OrganizationID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GroupAPI manages groups (/api/groups) and their account membership.
// SetMembers is an idempotent overwrite.
type GroupAPI interface {
	Create(ctx context.Context, group Group, members []string) (Group, error)
	Get(ctx context.Context, id string) (Group, error)
	List(ctx context.Context, opts ...ListOption) ([]Group, error)
	Delete(ctx context.Context, id string) error

	Members(ctx context.Context, id string) ([]string, error)
	SetMembers(ctx context.Context, id string, members []string) error
}

// ------------------------------------------------------------ HTTP

type groupHTTPClient struct {
	rest *restClient
}

func NewGroupHTTPClient(rest *restClient) *groupHTTPClient {
	return &groupHTTPClient{rest: rest}
}

// groupCreateDTO mirrors the server's create decode shape: the group body
// plus the synthetic "members" attribute seeding the initial membership.
type groupCreateDTO struct {
	api.GroupDTO
	RMembers []string `jsonapi:"attr,members,omitempty"`
}

func (c *groupHTTPClient) Create(ctx context.Context, group Group, members []string) (Group, error) {
	dto := &groupCreateDTO{GroupDTO: *groupToDTO(group), RMembers: members}
	created, err := restCreate[*api.GroupDTO](ctx, c.rest, "/api/groups", dto)
	if err != nil {
		return Group{}, err
	}
	return groupFromDTO(created), nil
}

func (c *groupHTTPClient) Get(ctx context.Context, id string) (Group, error) {
	dto, err := restGet[*api.GroupDTO](ctx, c.rest, "/api/groups/"+id)
	if err != nil {
		return Group{}, err
	}
	return groupFromDTO(dto), nil
}

func (c *groupHTTPClient) List(ctx context.Context, opts ...ListOption) ([]Group, error) {
	dtos, err := restList[*api.GroupDTO](ctx, c.rest, "/api/groups", opts...)
	if err != nil {
		return nil, err
	}
	groups := make([]Group, 0, len(dtos))
	for _, dto := range dtos {
		groups = append(groups, groupFromDTO(dto))
	}
	return groups, nil
}

func (c *groupHTTPClient) Delete(ctx context.Context, id string) error {
	return restDelete(ctx, c.rest, "/api/groups/"+id)
}

func (c *groupHTTPClient) Members(ctx context.Context, id string) ([]string, error) {
	var doc identifierDocument
	if err := restJSON(ctx, c.rest, http.MethodGet, "/api/groups/"+id+"/members", nil, &doc); err != nil {
		return nil, err
	}
	return doc.ids(), nil
}

func (c *groupHTTPClient) SetMembers(ctx context.Context, id string, members []string) error {
	doc := identifierDocumentOf("accounts", members)
	return restJSON(ctx, c.rest, http.MethodPost, "/api/groups/"+id+"/members", doc, nil)
}

func groupToDTO(group Group) *api.GroupDTO {
	dto := &api.GroupDTO{
		RRealmID:     group.RealmID,
		RName:        group.Name,
		RDescription: group.Description,
	}
	if group.OrganizationID != "" {
		dto.ROrganization = resource.RelationshipToDTO(resource.RelFromIDAndType(group.OrganizationID, "organizations"))
	}
	dto.RID = group.ID
	dto.RType = api.ResourceTypeGroup
	return dto
}

func groupFromDTO(dto *api.GroupDTO) Group {
	if dto == nil {
		return Group{}
	}
	group := Group{
		ID:          dto.ID(),
		RealmID:     dto.RRealmID,
		Name:        dto.RName,
		Description: dto.RDescription,
		CreatedAt:   dto.CreatedAt(),
		UpdatedAt:   dto.UpdatedAt(),
	}
	if dto.ROrganization != nil {
		group.OrganizationID = dto.ROrganization.ID()
	}
	return group
}
