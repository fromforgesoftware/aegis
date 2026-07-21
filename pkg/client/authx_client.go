package client

import (
	"context"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// Account is an Aegis account as returned by the authx surface.
type Account struct {
	ID            string
	RealmID       string
	Type          string
	Status        string
	Email         string
	EmailVerified bool
	DisplayName   string
	CreatedAt     time.Time
	LastLoginAt   time.Time
}

// Registration creates a password-based account in a realm.
type Registration struct {
	RealmID     string
	Email       string
	Password    string
	DisplayName string
}

// Credentials authenticates a password-based account.
type Credentials struct {
	RealmID  string
	Email    string
	Password string
}

// AuthxAPI is the first-party credential surface: registration, login and the
// email-verification / password-reset token flows.
type AuthxAPI interface {
	Register(ctx context.Context, registration Registration) (Account, error)
	Login(ctx context.Context, credentials Credentials) (Account, error)
	RequestEmailVerification(ctx context.Context, accountID string) error
	VerifyEmail(ctx context.Context, token string) error
	RequestPasswordReset(ctx context.Context, realmID, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
}

// ------------------------------------------------------------ GRPC

type passwordResetRequest struct {
	realmID string
	email   string
}

type confirmPasswordResetRequest struct {
	token       string
	newPassword string
}

type authxGRPCClient struct {
	registerEndpoint                 transport.Endpoint[Registration, Account]
	loginEndpoint                    transport.Endpoint[Credentials, Account]
	requestEmailVerificationEndpoint transport.Endpoint[string, struct{}]
	verifyEmailEndpoint              transport.Endpoint[string, struct{}]
	requestPasswordResetEndpoint     transport.Endpoint[passwordResetRequest, struct{}]
	confirmPasswordResetEndpoint     transport.Endpoint[confirmPasswordResetRequest, struct{}]
}

func NewAuthxGRPCClient(conn kitgrpc.Conn) *authxGRPCClient {
	return &authxGRPCClient{
		registerEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthxService_ServiceDesc, "Register",
			encodeRegisterRequest, decodeRegisterResponse, kitgrpc.ClientAuthMiddleware(),
		),
		loginEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthxService_ServiceDesc, "Login",
			encodeLoginRequest, decodeLoginResponse, kitgrpc.ClientAuthMiddleware(),
		),
		requestEmailVerificationEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthxService_ServiceDesc, "RequestEmailVerification",
			encodeRequestEmailVerificationRequest, decodeEmptyResponse[*aegisv1.RequestEmailVerificationResponse],
			kitgrpc.ClientAuthMiddleware(),
		),
		verifyEmailEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthxService_ServiceDesc, "VerifyEmail",
			encodeVerifyEmailRequest, decodeEmptyResponse[*aegisv1.VerifyEmailResponse],
			kitgrpc.ClientAuthMiddleware(),
		),
		requestPasswordResetEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthxService_ServiceDesc, "RequestPasswordReset",
			encodeRequestPasswordResetRequest, decodeEmptyResponse[*aegisv1.RequestPasswordResetResponse],
			kitgrpc.ClientAuthMiddleware(),
		),
		confirmPasswordResetEndpoint: kitgrpc.NewClientEndpoint(
			conn, aegisv1.AuthxService_ServiceDesc, "ConfirmPasswordReset",
			encodeConfirmPasswordResetRequest, decodeEmptyResponse[*aegisv1.ConfirmPasswordResetResponse],
			kitgrpc.ClientAuthMiddleware(),
		),
	}
}

func (c *authxGRPCClient) Register(ctx context.Context, registration Registration) (Account, error) {
	account, err := kitgrpc.Call(ctx, c.registerEndpoint, registration)
	if err != nil {
		return Account{}, apierrors.FromGRPCError(err)
	}
	return account, nil
}

func (c *authxGRPCClient) Login(ctx context.Context, credentials Credentials) (Account, error) {
	account, err := kitgrpc.Call(ctx, c.loginEndpoint, credentials)
	if err != nil {
		return Account{}, apierrors.FromGRPCError(err)
	}
	return account, nil
}

func (c *authxGRPCClient) RequestEmailVerification(ctx context.Context, accountID string) error {
	if _, err := kitgrpc.Call(ctx, c.requestEmailVerificationEndpoint, accountID); err != nil {
		return apierrors.FromGRPCError(err)
	}
	return nil
}

func (c *authxGRPCClient) VerifyEmail(ctx context.Context, token string) error {
	if _, err := kitgrpc.Call(ctx, c.verifyEmailEndpoint, token); err != nil {
		return apierrors.FromGRPCError(err)
	}
	return nil
}

func (c *authxGRPCClient) RequestPasswordReset(ctx context.Context, realmID, email string) error {
	if _, err := kitgrpc.Call(ctx, c.requestPasswordResetEndpoint, passwordResetRequest{realmID: realmID, email: email}); err != nil {
		return apierrors.FromGRPCError(err)
	}
	return nil
}

func (c *authxGRPCClient) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if _, err := kitgrpc.Call(ctx, c.confirmPasswordResetEndpoint, confirmPasswordResetRequest{token: token, newPassword: newPassword}); err != nil {
		return apierrors.FromGRPCError(err)
	}
	return nil
}

func encodeRegisterRequest(_ context.Context, registration Registration) (*aegisv1.RegisterRequest, error) {
	return &aegisv1.RegisterRequest{
		RealmId:     registration.RealmID,
		Email:       registration.Email,
		Password:    registration.Password,
		DisplayName: registration.DisplayName,
	}, nil
}

func decodeRegisterResponse(_ context.Context, resp *aegisv1.RegisterResponse) (Account, error) {
	return accountFromProto(resp.GetAccount()), nil
}

func encodeLoginRequest(_ context.Context, credentials Credentials) (*aegisv1.LoginRequest, error) {
	return &aegisv1.LoginRequest{
		RealmId:  credentials.RealmID,
		Email:    credentials.Email,
		Password: credentials.Password,
	}, nil
}

func decodeLoginResponse(_ context.Context, resp *aegisv1.LoginResponse) (Account, error) {
	return accountFromProto(resp.GetAccount()), nil
}

func encodeRequestEmailVerificationRequest(_ context.Context, accountID string) (*aegisv1.RequestEmailVerificationRequest, error) {
	return &aegisv1.RequestEmailVerificationRequest{AccountId: accountID}, nil
}

func encodeVerifyEmailRequest(_ context.Context, token string) (*aegisv1.VerifyEmailRequest, error) {
	return &aegisv1.VerifyEmailRequest{Token: token}, nil
}

func encodeRequestPasswordResetRequest(_ context.Context, req passwordResetRequest) (*aegisv1.RequestPasswordResetRequest, error) {
	return &aegisv1.RequestPasswordResetRequest{RealmId: req.realmID, Email: req.email}, nil
}

func encodeConfirmPasswordResetRequest(_ context.Context, req confirmPasswordResetRequest) (*aegisv1.ConfirmPasswordResetRequest, error) {
	return &aegisv1.ConfirmPasswordResetRequest{Token: req.token, NewPassword: req.newPassword}, nil
}

// decodeEmptyResponse discards a response message that carries no fields.
func decodeEmptyResponse[T any](_ context.Context, _ T) (struct{}, error) {
	return struct{}{}, nil
}

func accountFromProto(account *aegisv1.Account) Account {
	if account == nil {
		return Account{}
	}
	out := Account{
		ID:            account.GetId(),
		RealmID:       account.GetRealmId(),
		Type:          account.GetType(),
		Status:        account.GetStatus(),
		Email:         account.GetEmail(),
		EmailVerified: account.GetEmailVerified(),
		DisplayName:   account.GetDisplayName(),
	}
	if ts := account.GetCreatedAt(); ts != nil {
		out.CreatedAt = ts.AsTime()
	}
	if ts := account.GetLastLoginAt(); ts != nil {
		out.LastLoginAt = ts.AsTime()
	}
	return out
}
