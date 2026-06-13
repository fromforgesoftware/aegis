// Package internal wires Aegis's components into a single fx module
// that cmd/server composes alongside the kit's defaults.
package internal

import (
	"context"
	"net/http"
	"os"
	"time"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence"
	outboxpg "github.com/fromforgesoftware/go-kit/outbox/postgres"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/cryptox"
	"github.com/fromforgesoftware/aegis/internal/db"
	aegisgrpc "github.com/fromforgesoftware/aegis/internal/transport/grpc"
	aegishttp "github.com/fromforgesoftware/aegis/internal/transport/http"
)

// sweepInterval is how often the background sweeper removes expired grants and
// refreshes the projection.
const sweepInterval = time.Minute

// Stateful-session tuning: the purger reclaims session-state rows idle beyond
// sessionIdleTTL, and only runs when AEGIS_SESSIONS_STATEFUL is enabled.
const (
	sessionPurgeInterval = 5 * time.Minute
	sessionIdleTTL       = 30 * time.Minute
)

func statefulSessionsEnabled() bool {
	return os.Getenv("AEGIS_SESSIONS_STATEFUL") == "true"
}

// Version is the running Aegis version; matches the published image tag.
const Version = "0.1.0"

// FxModule wires Aegis: the db layer supplies repositories, the app
// layer composes usecases against them, and the transport layer exposes
// the gRPC surface. The persistence client (gormpg) is supplied by
// cmd/server so the whole graph shares one *gormdb.DBClient.
func FxModule() fx.Option {
	return fx.Module("aegis",
		repositoriesFxModule(),
		usecasesFxModule(),
		transportFxModule(),
		fx.Invoke(registerBootstrap),
	)
}

func repositoriesFxModule() fx.Option {
	return fx.Module("aegis:repositories",
		fx.Provide(
			fx.Annotate(db.NewAccountRepository, fx.As(new(app.AccountRepository))),
			fx.Annotate(db.NewCredentialRepository, fx.As(new(app.CredentialRepository))),
			fx.Annotate(db.NewPasswordPolicyRepository, fx.As(new(app.PasswordPolicyRepository))),
			fx.Annotate(db.NewVerificationTokenRepository, fx.As(new(app.VerificationTokenRepository))),
			fx.Annotate(db.NewPasswordResetTokenRepository, fx.As(new(app.PasswordResetTokenRepository))),
			fx.Annotate(db.NewFlowRepository, fx.As(new(app.FlowRepository))),
			fx.Annotate(db.NewSigningKeyRepository, fx.As(new(app.SigningKeyRepository))),
			fx.Annotate(db.NewClientRepository, fx.As(new(app.ClientRepository))),
			fx.Annotate(db.NewSessionRepository, fx.As(new(app.SessionRepository))),
			fx.Annotate(db.NewAuthorizationCodeRepository, fx.As(new(app.AuthorizationCodeRepository))),
			fx.Annotate(db.NewRefreshTokenRepository, fx.As(new(app.RefreshTokenRepository))),
			fx.Annotate(db.NewExternalIDPConfigRepository, fx.As(new(app.ExternalIDPConfigRepository))),
			fx.Annotate(db.NewAccountExternalIDRepository, fx.As(new(app.AccountExternalIDRepository))),
			fx.Annotate(db.NewPermissionRepository, fx.As(new(app.PermissionRepository))),
			fx.Annotate(db.NewRoleRepository, fx.As(new(app.RoleRepository))),
			fx.Annotate(db.NewRolePermissionRepository, fx.As(new(app.RolePermissionRepository))),
			fx.Annotate(db.NewAuthzResourceRepository, fx.As(new(app.AuthzResourceRepository))),
			fx.Annotate(db.NewGroupRepository, fx.As(new(app.GroupRepository))),
			fx.Annotate(db.NewGroupMemberRepository, fx.As(new(app.GroupMemberRepository))),
			fx.Annotate(db.NewBindingRepository, fx.As(new(app.BindingRepository))),
			fx.Annotate(db.NewAuthorizationProjectionRepository, fx.As(new(app.AuthorizationProjectionRepository))),
			fx.Annotate(db.NewEffectiveAuthorizationRepository, fx.As(new(app.AuthorizationReader))),
			fx.Annotate(db.NewPermissionInheritanceRepository, fx.As(new(app.PermissionInheritanceRepository))),
			fx.Annotate(db.NewRoleCompositionRepository, fx.As(new(app.RoleCompositionRepository))),
			fx.Annotate(db.NewRoleEffectivePermissionRepository, fx.As(new(app.RoleEffectivePermissionRepository))),
			fx.Annotate(db.NewAuthzVersionRepository, fx.As(new(app.VersionRepository))),
			fx.Annotate(db.NewSessionStateRepository, fx.As(new(app.SessionStateRepository))),
			fx.Annotate(db.NewQuotaPolicyRepository, fx.As(new(app.QuotaPolicyRepository))),
			fx.Annotate(db.NewMFAEnrollmentRepository, fx.As(new(app.MFAEnrollmentRepository))),
			fx.Annotate(db.NewRecoveryCodeRepository, fx.As(new(app.RecoveryCodeRepository))),
			fx.Annotate(db.NewStepUpTokenRepository, fx.As(new(app.StepUpTokenRepository))),
			fx.Annotate(db.NewRealmACRPolicyRepository, fx.As(new(app.RealmACRPolicyRepository))),
			fx.Annotate(db.NewRealmRepository, fx.As(new(app.RealmRepository))),
			fx.Annotate(db.NewOrganizationRepository, fx.As(new(app.OrganizationRepository))),
			fx.Annotate(db.NewAccountActiveOrgRepository, fx.As(new(app.AccountActiveOrgRepository))),
			fx.Annotate(db.NewAuditEventReadRepository, fx.As(new(app.AuditEventReader))),
			fx.Annotate(db.NewInvitationRepository, fx.As(new(app.InvitationRepository))),
			fx.Annotate(cryptox.NewCipherFromEnv, fx.As(new(app.KeyCipher))),
		),
	)
}

func usecasesFxModule() fx.Option {
	return fx.Module("aegis:usecases",
		fx.Provide(
			fx.Annotate(app.NewArgon2idHasher, fx.As(new(app.PasswordHasher))),
			fx.Annotate(app.NewLogNotificationSender, fx.As(new(app.NotificationSender))),
			fx.Annotate(newAuthxUsecase, fx.As(new(app.AuthxUsecase))),
			fx.Annotate(app.NewVerificationUsecase, fx.As(new(app.VerificationUsecase))),
			fx.Annotate(app.NewPasswordResetUsecase, fx.As(new(app.PasswordResetUsecase))),
			fx.Annotate(app.NewFlowUsecase, fx.As(new(app.FlowUsecase))),
			fx.Annotate(app.NewSigningKeyService, fx.As(new(app.SigningKeyService))),
			fx.Annotate(app.NewClientUsecase, fx.As(new(app.ClientUsecase))),
			fx.Annotate(app.NewOAuthUsecase, fx.As(new(app.OAuthUsecase))),
			fx.Annotate(app.NewExternalIDPConfigUsecase, fx.As(new(app.ExternalIDPConfigUsecase))),
			fx.Annotate(app.NewIdentityBrokerUsecase, fx.As(new(app.IdentityBrokerUsecase))),
			fx.Annotate(app.NewPermissionUsecase, fx.As(new(app.PermissionUsecase))),
			fx.Annotate(app.NewRoleUsecase, fx.As(new(app.RoleUsecase))),
			fx.Annotate(app.NewAuthzResourceUsecase, fx.As(new(app.AuthzResourceUsecase))),
			fx.Annotate(app.NewGroupUsecase, fx.As(new(app.GroupUsecase))),
			fx.Annotate(app.NewOrganizationUsecase, fx.As(new(app.OrganizationUsecase)), fx.As(new(app.ActiveOrgResolver))),
			fx.Annotate(db.NewAvatarStore, fx.As(new(app.AvatarStore))),
			fx.Annotate(app.NewAvatarUsecase, fx.As(new(app.AvatarUsecase))),
			fx.Annotate(newAuditSink, fx.As(new(app.AuditSink))),
			fx.Annotate(app.NewAuditor, fx.As(new(app.Auditor))),
			fx.Annotate(app.NewBindingUsecase, fx.As(new(app.BindingUsecase))),
			fx.Annotate(app.NewRoleResolver, fx.As(new(app.RoleResolver))),
			fx.Annotate(app.NewAuthorizationUsecase, fx.As(new(app.AuthorizationUsecase))),
			fx.Annotate(app.NewGrantSweeper, fx.As(new(app.GrantSweeper))),
			fx.Annotate(app.NewAccountModerationUsecase, fx.As(new(app.AccountModerationUsecase))),
			fx.Annotate(app.NewBanSweeper, fx.As(new(app.BanSweeper))),
			fx.Annotate(db.NewAccountMergeRepository, fx.As(new(app.AccountMergeRepository))),
			fx.Annotate(app.NewAccountMergeUsecase, fx.As(new(app.AccountMergeUsecase))),
			fx.Annotate(db.NewMagicLinkTokenRepository, fx.As(new(app.MagicLinkTokenRepository))),
			fx.Annotate(app.NewMagicLinkUsecase, fx.As(new(app.MagicLinkUsecase))),
			fx.Annotate(db.NewServiceAccountRepository, fx.As(new(app.ServiceAccountRepository))),
			fx.Annotate(app.NewServiceAccountUsecase, fx.As(new(app.ServiceAccountUsecase))),
			fx.Annotate(db.NewLoginSignalRepository, fx.As(new(app.LoginSignalRepository))),
			fx.Annotate(db.NewRealmRiskPolicyRepository, fx.As(new(app.RealmRiskPolicyRepository))),
			fx.Annotate(app.NewRiskPolicyUsecase, fx.As(new(app.RiskPolicyUsecase))),
			fx.Annotate(app.NewRiskUsecase, fx.As(new(app.RiskUsecase))),
			fx.Annotate(app.NewSessionStateUsecase, fx.As(new(app.SessionStateUsecase))),
			fx.Annotate(app.NewQuotaUsecase, fx.As(new(app.QuotaUsecase))),
			fx.Annotate(app.NewMFAUsecase, fx.As(new(app.MFAUsecase))),
			fx.Annotate(app.NewMFAPolicyUsecase, fx.As(new(app.MFAPolicyUsecase))),
			fx.Annotate(app.NewRealmUsecase, fx.As(new(app.RealmUsecase))),
			fx.Annotate(app.NewAuditQueryUsecase, fx.As(new(app.AuditQueryUsecase))),
			fx.Annotate(app.NewInvitationUsecase, fx.As(new(app.InvitationUsecase))),
			newDefaultHTTPDoer,
			fx.Annotate(app.NewOIDCConnector,
				fx.As(new(app.Connector)), fx.ResultTags(`group:"aegis:connectors"`)),
			fx.Annotate(app.NewGoogleConnector,
				fx.As(new(app.Connector)), fx.ResultTags(`group:"aegis:connectors"`)),
			fx.Annotate(app.NewFirebaseConnector,
				fx.As(new(app.Connector)), fx.ResultTags(`group:"aegis:connectors"`)),
			fx.Annotate(app.NewAppleConnector,
				fx.As(new(app.Connector)), fx.ResultTags(`group:"aegis:connectors"`)),
			fx.Annotate(app.NewGitHubConnector,
				fx.As(new(app.Connector)), fx.ResultTags(`group:"aegis:connectors"`)),
			fx.Annotate(app.NewConnectors, fx.ParamTags(`group:"aegis:connectors"`)),
			fx.Annotate(app.NewTokenIssuer, fx.As(new(app.TokenIssuer))),
		),
	)
}

func transportFxModule() fx.Option {
	return fx.Module("aegis:transport",
		kitrest.NewFxMiddleware(kitrest.NewGatewayMiddleware),
		fx.Invoke(registerGrantSweeper),
		fx.Invoke(registerSessionPurger),
		fx.Invoke(registerBanSweeper),
		fx.Provide(newAuthRateLimiter),
		kitgrpc.NewFxController(aegisgrpc.NewAdminController),
		kitgrpc.NewFxController(aegisgrpc.NewAuthxController),
		kitgrpc.NewFxController(aegisgrpc.NewOAuthController),
		kitgrpc.NewFxController(aegisgrpc.NewIdentityBrokerController),
		kitgrpc.NewFxController(aegisgrpc.NewAuthorizerController),
		kitgrpc.NewFxController(aegisgrpc.NewMFAController),
		kitrest.NewFxController(aegishttp.NewFlowController),
		kitrest.NewFxController(aegishttp.NewPagesController),
		kitrest.NewFxController(aegishttp.NewOIDCController),
		kitrest.NewFxController(aegishttp.NewClientController),
		kitrest.NewFxController(aegishttp.NewOAuthController),
		kitrest.NewFxController(aegishttp.NewExternalIDPController),
		kitrest.NewFxController(aegishttp.NewFederatedSignInController),
		kitrest.NewFxController(aegishttp.NewPermissionController),
		kitrest.NewFxController(aegishttp.NewRoleController),
		kitrest.NewFxController(aegishttp.NewAuthzResourceController),
		kitrest.NewFxController(aegishttp.NewGroupController),
		kitrest.NewFxController(aegishttp.NewOrganizationController),
		kitrest.NewFxController(aegishttp.NewAvatarController),
		kitrest.NewFxController(aegishttp.NewBindingController),
		kitrest.NewFxController(aegishttp.NewAuthorizationController),
		kitrest.NewFxController(aegishttp.NewSessionStateController),
		kitrest.NewFxController(aegishttp.NewQuotaPolicyController),
		kitrest.NewFxController(aegishttp.NewMFAController),
		kitrest.NewFxController(aegishttp.NewRealmACRPolicyController),
		kitrest.NewFxController(aegishttp.NewRealmController),
		kitrest.NewFxController(aegishttp.NewAuditEventController),
		kitrest.NewFxController(aegishttp.NewInvitationController),
		kitrest.NewFxController(aegishttp.NewAccountModerationController),
		kitrest.NewFxController(aegishttp.NewMagicLinkController),
		kitrest.NewFxController(aegishttp.NewServiceAccountController),
		kitrest.NewFxController(aegishttp.NewRiskController),
		kitrest.NewFxController(aegishttp.NewRealmRiskPolicyController),
	)
}

// registerGrantSweeper runs the expired-grant sweeper on an interval for the
// life of the process; OnStop cancels the loop.
func registerGrantSweeper(lc fx.Lifecycle, sweeper app.GrantSweeper) {
	ctx, cancel := context.WithCancel(context.Background())
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go runGrantSweeper(ctx, sweeper, log)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func runGrantSweeper(ctx context.Context, sweeper app.GrantSweeper, log logger.Logger) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := sweeper.Sweep(ctx); err != nil {
				log.ErrorContext(ctx, "grant sweep failed", "error", err)
			}
		}
	}
}

// registerBanSweeper runs the ban-expiry sweeper on an interval so timed bans
// lift themselves; OnStop cancels the loop.
func registerBanSweeper(lc fx.Lifecycle, sweeper app.BanSweeper) {
	ctx, cancel := context.WithCancel(context.Background())
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go runBanSweeper(ctx, sweeper, log)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func runBanSweeper(ctx context.Context, sweeper app.BanSweeper, log logger.Logger) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := sweeper.Sweep(ctx); err != nil {
				log.ErrorContext(ctx, "ban sweep failed", "error", err)
			}
		}
	}
}

// registerSessionPurger runs the idle-session sweeper only when stateful
// sessions are enabled; otherwise the table stays cold and off the hot path.
func registerSessionPurger(lc fx.Lifecycle, states app.SessionStateUsecase) {
	if !statefulSessionsEnabled() {
		return
	}
	purger := app.NewSessionPurger(states, sessionIdleTTL)
	ctx, cancel := context.WithCancel(context.Background())
	log := logger.New()
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go runSessionPurger(ctx, purger, log)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func runSessionPurger(ctx context.Context, purger app.SessionPurger, log logger.Logger) {
	ticker := time.NewTicker(sessionPurgeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := purger.Purge(ctx); err != nil {
				log.ErrorContext(ctx, "session purge failed", "error", err)
			}
		}
	}
}

// newAuditSink selects the built-in audit sink. Postgres (append-only,
// partitioned) is the zero-config default; AEGIS_AUDIT_SINK=stdout switches to
// structured-log emission, and AEGIS_AUDIT_SINK=outbox writes to the
// transactional outbox for a talos drainer to forward.
func newAuditSink(client *gormdb.DBClient) app.AuditSink {
	switch os.Getenv("AEGIS_AUDIT_SINK") {
	case "stdout":
		return app.NewStdoutAuditSink()
	case "outbox":
		return db.NewAuditOutboxSink(outboxpg.New(client, "aegis.outbox"))
	default:
		return db.NewAuditEventSink(client)
	}
}

// newDefaultHTTPDoer is the upstream-fetch client the identity-broker
// connectors share (5s timeout per call).
func newDefaultHTTPDoer() app.HTTPDoer {
	return &http.Client{Timeout: 5 * time.Second}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newAuthxUsecase wires the authx usecase with risk-based auth (fx can't supply
// the variadic option directly).
func newAuthxUsecase(
	accounts app.AccountRepository,
	creds app.CredentialRepository,
	policies app.PasswordPolicyRepository,
	hasher app.PasswordHasher,
	tx persistence.Transactioner,
	risk app.RiskUsecase,
) app.AuthxUsecase {
	return app.NewAuthxUsecase(accounts, creds, policies, hasher, tx, app.WithRisk(risk))
}

// newAuthRateLimiter is the shared per-IP token bucket guarding the
// credential-submit endpoints (flow submit + hosted login POST): a burst of
// 10 then ~1/sec sustained, per source IP. In-memory per instance — a
// distributed limiter can replace it via the same kit middleware seam when
// the cache stack lands.
func newAuthRateLimiter() *kitrest.RateLimitMiddleware {
	return kitrest.NewRateLimitMiddleware(1, 10)
}
