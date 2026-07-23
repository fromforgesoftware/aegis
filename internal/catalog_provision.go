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

// registerCatalogProvisioning applies the mounted catalog documents at
// startup, Grafana-provisioning style: the deployment renders the catalogs
// into a config file (AEGIS_CATALOGS_FILE, a JSON array of documents) and a
// checksum annotation rolls the pods when it changes, so editing the values is
// what deploys the change. Unlike the admin bootstrap this is FATAL on error —
// an invalid catalog must fail readiness and halt the rollout rather than
// serve half-applied vocabulary; the previous pods keep serving the old one.
func registerCatalogProvisioning(lc fx.Lifecycle, catalogs app.CatalogUsecase) {
	path := os.Getenv("AEGIS_CATALOGS_FILE")
	if path == "" {
		return
	}
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			docs, err := loadCatalogDocuments(path)
			if err != nil {
				return err
			}
			if err := catalogs.Apply(context.Background(), docs); err != nil {
				return fmt.Errorf("catalog provisioning: %w", err)
			}
			log.Info("catalog provisioning applied", "file", path, "catalogs", len(docs))
			return nil
		},
	})
}

func loadCatalogDocuments(path string) ([]domain.CatalogDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalog provisioning: read %s: %w", path, err)
	}
	var docs []domain.CatalogDocument
	if err := json.Unmarshal(raw, &docs); err != nil {
		return nil, fmt.Errorf("catalog provisioning: parse %s: %w", path, err)
	}
	return docs, nil
}
