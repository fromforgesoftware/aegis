package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestValidatePassword(t *testing.T) {
	strict := domain.NewPasswordPolicy(
		domain.WithPasswordPolicyMinLength(10),
		domain.WithPasswordPolicyMaxLength(20),
		domain.WithPasswordPolicyRequireUppercase(true),
		domain.WithPasswordPolicyRequireLowercase(true),
		domain.WithPasswordPolicyRequireDigit(true),
		domain.WithPasswordPolicyRequireSymbol(true),
	)

	cases := []struct {
		name     string
		policy   domain.PasswordPolicy
		password string
		wantErr  bool
	}{
		{"default accepts 8+ chars", domain.DefaultPasswordPolicy(), "password", false},
		{"default rejects 7 chars", domain.DefaultPasswordPolicy(), "passwor", true},
		{"default ignores complexity", domain.DefaultPasswordPolicy(), "lowercaseonly", false},
		{"min length boundary ok", domain.NewPasswordPolicy(domain.WithPasswordPolicyMinLength(5)), "12345", false},
		{"below min length", domain.NewPasswordPolicy(domain.WithPasswordPolicyMinLength(5)), "1234", true},
		{"max length exceeded", domain.NewPasswordPolicy(domain.WithPasswordPolicyMinLength(1), domain.WithPasswordPolicyMaxLength(4)), "12345", true},
		{"max length zero means no max", domain.NewPasswordPolicy(domain.WithPasswordPolicyMaxLength(0)), "a very long passphrase indeed", false},
		{"requires uppercase missing", domain.NewPasswordPolicy(domain.WithPasswordPolicyRequireUppercase(true)), "lower123!", true},
		{"requires lowercase missing", domain.NewPasswordPolicy(domain.WithPasswordPolicyRequireLowercase(true)), "UPPER123!", true},
		{"requires digit missing", domain.NewPasswordPolicy(domain.WithPasswordPolicyRequireDigit(true)), "NoDigits!", true},
		{"requires symbol missing", domain.NewPasswordPolicy(domain.WithPasswordPolicyRequireSymbol(true)), "NoSymbol123", true},
		{"strict accepts strong password", strict, "Str0ng!Pass", false},
		{"strict rejects without symbol", strict, "Str0ngPassXY", true},
		{"strict rejects too long", strict, "Str0ng!PassWordWayTooLong", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidatePassword(tc.policy, tc.password)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
