# aegis — authorization, identity & OAuth2

A forge-platform service, split from the forge monorepo (Grafana-style: one repo
per product). Consumes the published `github.com/fromforgesoftware/go-kit`.

## Commands
- Build: `go build ./...`
- Unit tests: `go test ./...`
- Integration tests (need Postgres): `go test -tags=integration ./...`
- Lint: `golangci-lint run`
- Regenerate protobuf after editing `api/proto/*.proto`: `buf generate`
- DB migrate: run `./cmd/migrator`

## Stack
Go 1.25 · go-kit (transport, persistence/GORM, jsonapi, auth, audit, outbox) ·
PostgreSQL · gRPC + JSON:API REST · buf (protobuf).

## Structure (capabilities, not a file map)
- `internal/domain` — resource entities implementing go-kit `resource.Resource`
- `internal/app` — usecases + ports (interfaces)
- `internal/db` — Postgres repositories + `cmd/migrator/migrations`
- `internal/transport/{http,grpc}` — JSON:API + gRPC controllers
- `api/proto` → generated into `pkg/api`; `pkg/client` is the consumer SDK
- `cmd/{server,migrator}` — binaries · `deploy/helm` — chart

## Conventions
- Commits: `<type>(<scope>): <subject>` — ONE line, ≤72 chars, lowercase subject,
  no body/footer, no Co-Authored-By trailer.
- REST surface is JSON:API (go-kit `NewJsonApi*` handlers + `resource.RestDTO`);
  never hand-rolled plain-JSON structs.
- Repository pattern: resource-interface domain; the entity implements it; use
  the generic go-kit `repository.Getter/Lister` driven by `search.Option`.
- Tests: testify with precise `mock.MatchedBy` (not blanket `mock.Anything`);
  unit = `*_test.go`, integration = `*_integration_test.go` + `//go:build integration`.

## Boundaries (always / never)
- NEVER commit secrets. Helm `secret.yaml` holds values placeholders only.
- NEVER hand-edit generated `pkg/api/**/*.pb.go` — edit the `.proto` and run
  `buf generate`. (A blind module-path sed corrupts proto `rawDesc` descriptors.)
- Don't add dependabot.
- Service deps integrate via published modules / APIs, never relative `replace`.

## Platform context
Issues realm JWTs, manages realms/orgs/roles/bindings, OAuth2. Emits audit events to the go-kit `audit.Sink` port — `AEGIS_AUDIT_SINK` selects db (default) / stdout / outbox. It does NOT import any telemetry backend; the talos drainer forwards the outbox. Other services verify aegis-issued JWTs via go-kit auth.
