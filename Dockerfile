# Aegis builds two binaries (server + migrator) into one distroless image.
# Build context is the forge repo root so the kit replace directive
# (../../go/kit) resolves. See skaffold.yaml / `docker build -f`.
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src

# The module surface Aegis needs: the kit, the service itself, and Hallmark
# (the audit sink imports hallmark/pkg/client via a ../hallmark replace).
COPY services/aegis/    /src/services/aegis/

WORKDIR /src/services/aegis
ENV GOWORK=off
RUN CGO_ENABLED=0 go build -trimpath -o /out/server   ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -o /out/migrator ./cmd/migrator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/server   /app/server
COPY --from=builder /out/migrator /app/migrator
# 8080 = REST/OpenAPI, 9090 = gRPC
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
