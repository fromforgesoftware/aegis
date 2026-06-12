# Aegis builds two binaries (server + migrator) into one distroless image.
# Standalone repo: the module lives at the repo root and consumes the
# published go-kit module (no replace directives, no monorepo siblings).
# Build context is this repo's root.
ARG GO_VERSION=1.25
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder
ARG TARGETOS TARGETARCH
WORKDIR /src

# Disable workspace mode so the build relies solely on go.mod/go.sum.
ENV GOWORK=off

# Resolve dependencies first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the module source (migrations are embedded via go:embed).
COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/server   ./cmd/server
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/migrator ./cmd/migrator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/server   /app/server
COPY --from=builder /out/migrator /app/migrator
# 8080 = REST/OpenAPI, 9090 = gRPC
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
