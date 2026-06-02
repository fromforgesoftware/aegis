package http_test

import (
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/app/apptest"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// resolvingRealms is a RealmUsecase stub that resolves any realm name to a realm
// whose ID is the given value — the name→UUID step the protocol controllers
// perform. .Maybe() since some handlers error before reaching resolution.
func resolvingRealms(t *testing.T, id string) app.RealmUsecase {
	r := apptest.NewRealmUsecase(t)
	r.EXPECT().Get(mock.Anything, mock.Anything).
		Return(domain.NewRealm("realm", domain.WithRealmID(id)), nil).Maybe()
	return r
}
