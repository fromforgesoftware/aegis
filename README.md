# aegis

Forge authorization, identity, and OAuth2 service (REST + gRPC). Emits audit
events to a `go-kit/audit.Sink` (db / stdout / outbox); telemetry forwarding is
out-of-process via talos. Named for the shield of Zeus/Athena.
