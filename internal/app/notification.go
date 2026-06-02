package app

import (
	"context"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

// NotificationSender delivers transactional auth messages (verification,
// reset, magic-link, invitations). It is a port: the built-in
// implementation is a zero-config fallback, and consumers can route to the
// @forge/herald service later by supplying an adapter that satisfies it.
type NotificationSender interface {
	SendEmailVerification(ctx context.Context, to, token string) error
	SendPasswordReset(ctx context.Context, to, token string) error
	SendInvitation(ctx context.Context, to, token string) error
	SendMagicLink(ctx context.Context, to, token string) error
}

// LogNotificationSender is the built-in fallback: it logs the message
// instead of delivering it, so the service runs with no mail infra. Real
// SMTP/SES and the @forge/herald adapter are follow-ups. Dev-only — it
// logs the raw token so a developer can complete the flow locally.
type LogNotificationSender struct {
	log logger.Logger
}

func NewLogNotificationSender(log logger.Logger) *LogNotificationSender {
	return &LogNotificationSender{log: log}
}

func (s *LogNotificationSender) SendEmailVerification(_ context.Context, to, token string) error {
	s.log.WithKeysAndValues("to", to, "token", token).
		Info("email verification not delivered (built-in log sender)")
	return nil
}

func (s *LogNotificationSender) SendPasswordReset(_ context.Context, to, token string) error {
	s.log.WithKeysAndValues("to", to, "token", token).
		Info("password reset not delivered (built-in log sender)")
	return nil
}

func (s *LogNotificationSender) SendInvitation(_ context.Context, to, token string) error {
	s.log.WithKeysAndValues("to", to, "token", token).
		Info("invitation not delivered (built-in log sender)")
	return nil
}

func (s *LogNotificationSender) SendMagicLink(_ context.Context, to, token string) error {
	s.log.WithKeysAndValues("to", to, "token", token).
		Info("magic link not delivered (built-in log sender)")
	return nil
}
