package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

// registerPreferenceProvisioning applies the mounted preference document at startup, the
// same way the catalog is applied: the deployment renders `preferences` from the values
// into a config file (AEGIS_PREFERENCES_FILE) and a checksum annotation rolls the pods when
// it changes, so editing the values file is what deploys a new preference.
//
// FATAL on error, deliberately, and for the same reason the catalog is. Two failures are
// possible and both must stop the rollout rather than be logged past:
//
//   - An invalid document would leave a realm serving half a key space, so clients would see
//     controls appear and disappear depending on which pod answered.
//   - A document that dropped a key people have stored values for would DELETE those values
//     via the spec's cascade. The prune gate refuses that, and the refusal has to fail
//     readiness — the previous pods keep serving the previous key space, and the operator
//     either restores the key or adds it to `force` on purpose.
func registerPreferenceProvisioning(
	lc fx.Lifecycle, preferences app.PreferenceUsecase, realms app.RealmUsecase,
) {
	path := os.Getenv("AEGIS_PREFERENCES_FILE")
	if path == "" {
		return
	}
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			doc, err := loadPreferenceDocument(path)
			if err != nil {
				return err
			}
			ctx := context.Background()

			// The document names its realm, or inherits the bootstrap realm — which is the
			// single-product case and the common one. Resolving by NAME rather than id is
			// what lets the values file stay free of uuids.
			name := doc.Realm
			if name == "" {
				name = os.Getenv("AEGIS_BOOTSTRAP_REALM")
			}
			if name == "" {
				return fmt.Errorf(
					"preference provisioning: %s names no realm and AEGIS_BOOTSTRAP_REALM is unset", path)
			}
			realm, err := realms.Get(ctx, app.RealmByName(name))
			if err != nil {
				return fmt.Errorf("preference provisioning: realm %q: %w", name, err)
			}
			if realm == nil {
				return fmt.Errorf("preference provisioning: realm %q does not exist", name)
			}

			if err := preferences.Apply(ctx, realm.ID(), doc); err != nil {
				return fmt.Errorf("preference provisioning: %w", err)
			}
			log.Info("preference provisioning applied",
				"file", path, "realm", name, "specs", len(doc.Specs))
			return nil
		},
	})
}

// loadPreferenceDocument reads the document, accepting either shape the values file can
// produce.
//
// A bare LIST of specs is accepted alongside the full object because that is what
// `preferences: [ ... ]` renders to, and it is the shape a product reaches for first. The
// object form is what a product needs once it has to name a realm or force a removal.
// Accepting both keeps the simple case simple without a second config key.
func loadPreferenceDocument(path string) (domain.PreferenceDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return domain.PreferenceDocument{}, fmt.Errorf("preference provisioning: read %s: %w", path, err)
	}

	var doc domain.PreferenceDocument
	if err := json.Unmarshal(raw, &doc); err == nil && len(doc.Specs) > 0 {
		return doc, nil
	}

	var specs []domain.PreferenceSpec
	if listErr := json.Unmarshal(raw, &specs); listErr != nil {
		// Report the OBJECT parse error, not the list one: the object form is the documented
		// shape, so its message is the more useful of the two for a malformed file.
		return domain.PreferenceDocument{}, fmt.Errorf(
			"preference provisioning: parse %s: expected a list of specs or an object with a "+
				"specs field", path)
	}
	return domain.PreferenceDocument{Specs: specs}, nil
}
