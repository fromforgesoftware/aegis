//go:build integration

// Package internaltest holds integration-test helpers for Aegis. It is
// behind the `integration` build tag so gnomock (which pulls docker libs
// with test-only CVEs) stays out of the production build + govulncheck.
package internaltest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fromforgesoftware/go-kit/migrator"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormdbtest"
	"github.com/stretchr/testify/require"
)

// GetDB returns a per-process singleton Postgres (via the kit's
// gormdbtest container helper) with Aegis's migrations applied by the
// real kit migrator.
//
// We let gormdbtest stand up the container but apply migrations with
// migrator.Up rather than gormdbtest's own migration option: gnomock
// v0.32 runs each WithQueriesFile in REVERSE order, which breaks a
// multi-file migration chain (0004 before 0001). The real migrator uses
// golang-migrate, so files run in order. DB_SCHEMA=aegis mirrors prod;
// the common-pre-migration bootstrap creates the aegis schema before
// golang-migrate's tracking table needs it.
func GetDB(t *testing.T) *gormdb.DBClient {
	t.Helper()

	tdb := gormdbtest.GetDB(t, "aegis")
	if tdb == nil {
		t.Skip("test database unavailable (docker/gnomock); skipping integration test")
	}

	t.Setenv("DB_HOST", tdb.Host)
	t.Setenv("DB_PORT", fmt.Sprintf("%d", tdb.Port))
	t.Setenv("DB_USER", tdb.User)
	t.Setenv("DB_PASSWORD", tdb.Password)
	t.Setenv("DB_NAME", tdb.DBName)
	t.Setenv("DB_SSL", "disable")
	t.Setenv("DB_SCHEMA", "aegis")

	// os.DirFS rooted at cmd/migrator exposes the "migrations" subdir the
	// migrator expects (it reads "migrations/*.up.sql").
	require.NoError(t, migrator.Up(context.Background(), os.DirFS(migratorDir()), migrator.WithServiceName("aegis")))
	return tdb.DBClient
}

// TruncateTables wipes Aegis's tables between tests sharing the singleton
// container. Children first to respect FKs.
func TruncateTables(t *testing.T, db *gormdb.DBClient) {
	t.Helper()
	stmt := `TRUNCATE TABLE aegis.password_credential,
	                       aegis.user_account,
	                       aegis.account,
	                       aegis.permission,
	                       aegis.audit_event,
	                       aegis.login_signal,
	                       aegis.realm_risk_policy,
	                       aegis.realm
	         RESTART IDENTITY CASCADE;`
	require.NoError(t, db.Exec(stmt).Error)
}

// migratorDir resolves services/aegis/cmd/migrator relative to this
// source file, independent of the test's working directory.
func migratorDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "cmd", "migrator")
}
