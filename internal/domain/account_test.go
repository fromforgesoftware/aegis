package domain_test

import (
	"testing"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestAccountTypeValid(t *testing.T) {
	cases := map[string]struct {
		in   domain.AccountType
		want bool
	}{
		"user":    {domain.AccountTypeUser, true},
		"service": {domain.AccountTypeService, true},
		"empty":   {domain.AccountType(""), false},
		"bogus":   {domain.AccountType("ADMIN"), false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.in.Valid(); got != c.want {
				t.Fatalf("Valid() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAccountStatusValid(t *testing.T) {
	cases := map[string]struct {
		in   domain.AccountStatus
		want bool
	}{
		"created":  {domain.AccountStatusCreated, true},
		"enabled":  {domain.AccountStatusEnabled, true},
		"disabled": {domain.AccountStatusDisabled, true},
		"banned":   {domain.AccountStatusBanned, true},
		"empty":    {domain.AccountStatus(""), false},
		"bogus":    {domain.AccountStatus("PENDING"), false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.in.Valid(); got != c.want {
				t.Fatalf("Valid() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNewAccountDefaults(t *testing.T) {
	a := domain.NewAccount("realm-1", "user@example.com", "User")

	if a.Type() != domain.ResourceTypeAccount {
		t.Fatalf("resource type = %q, want %q", a.Type(), domain.ResourceTypeAccount)
	}
	if a.RealmID() != "realm-1" {
		t.Fatalf("realm = %q", a.RealmID())
	}
	if a.Email() != "user@example.com" {
		t.Fatalf("email = %q", a.Email())
	}
	if a.DisplayName() != "User" {
		t.Fatalf("displayName = %q", a.DisplayName())
	}
	if a.AccountType() != domain.AccountTypeUser {
		t.Fatalf("default type = %q, want USER", a.AccountType())
	}
	if a.Status() != domain.AccountStatusEnabled {
		t.Fatalf("default status = %q, want ENABLED", a.Status())
	}
	if a.LastLoginAt() != nil {
		t.Fatal("lastLoginAt should be nil by default")
	}
}

func TestNewAccountOptions(t *testing.T) {
	a := domain.NewAccount("realm-1", "svc@example.com", "Svc",
		domain.WithAccountID("acc-123"),
		domain.WithAccountType(domain.AccountTypeService),
		domain.WithAccountStatus(domain.AccountStatusDisabled),
		domain.WithAccountEmailVerified(true),
	)

	if a.ID() != "acc-123" {
		t.Fatalf("id = %q, want acc-123", a.ID())
	}
	if a.AccountType() != domain.AccountTypeService {
		t.Fatalf("type = %q, want SERVICE", a.AccountType())
	}
	if a.Status() != domain.AccountStatusDisabled {
		t.Fatalf("status = %q, want DISABLED", a.Status())
	}
	if !a.EmailVerified() {
		t.Fatal("emailVerified should be true")
	}
}
