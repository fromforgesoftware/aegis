# Aegis service targets. Self-contained; can be run directly
# (`make -f services/aegis/Makefile.mk aegis-build`) or included by the
# root Makefile.
AEGIS_DIR ?= services/aegis

.PHONY: aegis-proto aegis-build aegis-test aegis-vet aegis-lint aegis-migrate aegis-run

aegis-proto:        ## Regenerate Go code from the Aegis protos
	cd $(AEGIS_DIR) && buf generate

aegis-lint:         ## Lint the Aegis protos
	cd $(AEGIS_DIR) && buf lint

aegis-build:        ## Build all Aegis packages
	cd $(AEGIS_DIR) && go build ./...

aegis-vet:          ## Vet the Aegis module
	cd $(AEGIS_DIR) && go vet ./...

aegis-test:         ## Run Aegis unit tests
	cd $(AEGIS_DIR) && go test ./...

# Local-dev environment for the bare binary. In k8s the Helm chart supplies
# these (deploy/helm/values.yaml is the source of truth); the app itself
# hardcodes no config.
AEGIS_LOCAL_ENV := SVC_NAME=aegis REST_ADDRESS=:8080 HTTP_ADDRESS=:8080 GRPC_ADDRESS=:9090

aegis-migrate:      ## Apply Aegis migrations (reads DB_* env)
	cd $(AEGIS_DIR) && go run ./cmd/migrator

aegis-run:          ## Run the Aegis server locally
	cd $(AEGIS_DIR) && $(AEGIS_LOCAL_ENV) go run ./cmd/server
