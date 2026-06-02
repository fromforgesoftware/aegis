package domain

import (
	"errors"
	"fmt"
	"unicode"

	"github.com/fromforgesoftware/go-kit/resource"
)

const ResourceTypePasswordPolicy resource.Type = "passwordPolicies"

// PasswordPolicy is a realm's password-strength rule: a minimum (and
// optional maximum) length plus optional character-class requirements.
// It is a resource, 1:1 with a realm (keyed by the realm id). A realm
// with no configured row uses DefaultPasswordPolicy.
type PasswordPolicy interface {
	resource.Resource
	MinLength() int
	MaxLength() int // 0 = no maximum
	RequireUppercase() bool
	RequireLowercase() bool
	RequireDigit() bool
	RequireSymbol() bool
}

type passwordPolicy struct {
	resource.Resource

	minLength        int
	maxLength        int
	requireUppercase bool
	requireLowercase bool
	requireDigit     bool
	requireSymbol    bool
}

type PasswordPolicyOption func(*passwordPolicy)

func WithPasswordPolicyRealmID(id string) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.Resource = resource.Update(p.Resource, resource.WithID(id)) }
}
func WithPasswordPolicyMinLength(n int) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.minLength = n }
}
func WithPasswordPolicyMaxLength(n int) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.maxLength = n }
}
func WithPasswordPolicyRequireUppercase(v bool) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.requireUppercase = v }
}
func WithPasswordPolicyRequireLowercase(v bool) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.requireLowercase = v }
}
func WithPasswordPolicyRequireDigit(v bool) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.requireDigit = v }
}
func WithPasswordPolicyRequireSymbol(v bool) PasswordPolicyOption {
	return func(p *passwordPolicy) { p.requireSymbol = v }
}

// NewPasswordPolicy builds a value-constructed policy (default min length
// 8, no character-class rules). The repo entity implements PasswordPolicy
// directly, so this is for the default and for tests, not for reads.
func NewPasswordPolicy(opts ...PasswordPolicyOption) PasswordPolicy {
	p := &passwordPolicy{
		Resource:  resource.New(resource.WithType(ResourceTypePasswordPolicy)),
		minLength: 8,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// DefaultPasswordPolicy is the global floor applied when a realm has no
// policy of its own: at least 8 characters, no character-class rules.
func DefaultPasswordPolicy() PasswordPolicy {
	return NewPasswordPolicy()
}

func (p *passwordPolicy) MinLength() int         { return p.minLength }
func (p *passwordPolicy) MaxLength() int         { return p.maxLength }
func (p *passwordPolicy) RequireUppercase() bool { return p.requireUppercase }
func (p *passwordPolicy) RequireLowercase() bool { return p.requireLowercase }
func (p *passwordPolicy) RequireDigit() bool     { return p.requireDigit }
func (p *passwordPolicy) RequireSymbol() bool    { return p.requireSymbol }

// ValidatePassword reports the first way password violates the policy, or
// nil when it satisfies every rule. It reads only the PasswordPolicy
// accessors, so it works on the repo entity, the default, and value-
// constructed policies alike.
func ValidatePassword(p PasswordPolicy, password string) error {
	if len(password) < p.MinLength() {
		return fmt.Errorf("password must be at least %d characters", p.MinLength())
	}
	if p.MaxLength() > 0 && len(password) > p.MaxLength() {
		return fmt.Errorf("password must be at most %d characters", p.MaxLength())
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}

	switch {
	case p.RequireUppercase() && !hasUpper:
		return errors.New("password must contain an uppercase letter")
	case p.RequireLowercase() && !hasLower:
		return errors.New("password must contain a lowercase letter")
	case p.RequireDigit() && !hasDigit:
		return errors.New("password must contain a digit")
	case p.RequireSymbol() && !hasSymbol:
		return errors.New("password must contain a symbol")
	}
	return nil
}
