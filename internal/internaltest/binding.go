package internaltest

import (
	"time"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

type BindingOption func(*bindingOpts)

type bindingOpts struct {
	id          string
	resourceID  string
	roleID      string
	subjectType domain.SubjectType
	subjectID   string
	expiresAt   *time.Time
}

func defaultBindingOptions() []BindingOption {
	return []BindingOption{
		WithBindingResourceID("res-test"),
		WithBindingRoleID("role-test"),
		WithBindingSubjectType(domain.SubjectTypeAccount),
		WithBindingSubjectID("acct-test"),
	}
}

func WithBindingID(id string) BindingOption { return func(o *bindingOpts) { o.id = id } }
func WithBindingResourceID(id string) BindingOption {
	return func(o *bindingOpts) { o.resourceID = id }
}
func WithBindingRoleID(id string) BindingOption { return func(o *bindingOpts) { o.roleID = id } }
func WithBindingSubjectType(t domain.SubjectType) BindingOption {
	return func(o *bindingOpts) { o.subjectType = t }
}
func WithBindingSubjectID(id string) BindingOption { return func(o *bindingOpts) { o.subjectID = id } }
func WithBindingExpiresAt(t *time.Time) BindingOption {
	return func(o *bindingOpts) { o.expiresAt = t }
}

func NewBinding(opts ...BindingOption) domain.Binding {
	o := &bindingOpts{}
	for _, opt := range append(defaultBindingOptions(), opts...) {
		opt(o)
	}
	domainOpts := []domain.BindingOption{domain.WithBindingExpiresAt(o.expiresAt)}
	if o.id != "" {
		domainOpts = append(domainOpts, domain.WithBindingID(o.id))
	}
	return domain.NewBinding(o.resourceID, o.roleID, o.subjectType, o.subjectID, domainOpts...)
}

// MatchBinding compares resource + role + subject, ignoring id/timestamps.
func MatchBinding(want domain.Binding) func(domain.Binding) bool {
	return func(got domain.Binding) bool {
		if want == nil {
			return got == nil
		}
		if got == nil {
			return false
		}
		return want.ResourceID() == got.ResourceID() &&
			want.RoleID() == got.RoleID() &&
			want.SubjectType() == got.SubjectType() &&
			want.SubjectID() == got.SubjectID()
	}
}
