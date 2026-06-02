package grpc

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"

	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

// peerIP extracts the client IP from the gRPC peer, feeding risk-based auth.
// Empty when the peer/address is unavailable (risk then skips this login).
func peerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	if host, _, err := net.SplitHostPort(p.Addr.String()); err == nil {
		return host
	}
	return p.Addr.String()
}

type authxController struct {
	authx         app.AuthxUsecase
	verification  app.VerificationUsecase
	passwordReset app.PasswordResetUsecase
}

// NewAuthxController builds the AuthxService controller. Returned errors
// are apierrors values; the kit's gRPC layer maps them to status codes.
func NewAuthxController(
	authx app.AuthxUsecase,
	verification app.VerificationUsecase,
	passwordReset app.PasswordResetUsecase,
) kitgrpc.Controller {
	return &authxController{authx: authx, verification: verification, passwordReset: passwordReset}
}

func (c *authxController) SD() kitgrpc.ServiceDesc {
	return &aegisv1.AuthxService_ServiceDesc
}

func (c *authxController) Register(ctx context.Context, req *aegisv1.RegisterRequest) (*aegisv1.RegisterResponse, error) {
	acc, err := c.authx.Register(ctx, app.RegisterInput{
		RealmID:     req.GetRealmId(),
		Email:       req.GetEmail(),
		Password:    req.GetPassword(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, err
	}
	return &aegisv1.RegisterResponse{Account: toProtoAccount(acc)}, nil
}

func (c *authxController) Login(ctx context.Context, req *aegisv1.LoginRequest) (*aegisv1.LoginResponse, error) {
	acc, err := c.authx.Login(ctx, app.LoginInput{
		RealmID:  req.GetRealmId(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
		IP:       peerIP(ctx),
	})
	if err != nil {
		return nil, err
	}
	return &aegisv1.LoginResponse{Account: toProtoAccount(acc)}, nil
}

func (c *authxController) RequestEmailVerification(ctx context.Context, req *aegisv1.RequestEmailVerificationRequest) (*aegisv1.RequestEmailVerificationResponse, error) {
	if err := c.verification.RequestEmailVerification(ctx, req.GetAccountId()); err != nil {
		return nil, err
	}
	return &aegisv1.RequestEmailVerificationResponse{}, nil
}

func (c *authxController) VerifyEmail(ctx context.Context, req *aegisv1.VerifyEmailRequest) (*aegisv1.VerifyEmailResponse, error) {
	if err := c.verification.VerifyEmail(ctx, req.GetToken()); err != nil {
		return nil, err
	}
	return &aegisv1.VerifyEmailResponse{}, nil
}

func (c *authxController) RequestPasswordReset(ctx context.Context, req *aegisv1.RequestPasswordResetRequest) (*aegisv1.RequestPasswordResetResponse, error) {
	if err := c.passwordReset.RequestPasswordReset(ctx, req.GetRealmId(), req.GetEmail()); err != nil {
		return nil, err
	}
	return &aegisv1.RequestPasswordResetResponse{}, nil
}

func (c *authxController) ConfirmPasswordReset(ctx context.Context, req *aegisv1.ConfirmPasswordResetRequest) (*aegisv1.ConfirmPasswordResetResponse, error) {
	if err := c.passwordReset.ConfirmPasswordReset(ctx, req.GetToken(), req.GetNewPassword()); err != nil {
		return nil, err
	}
	return &aegisv1.ConfirmPasswordResetResponse{}, nil
}

func toProtoAccount(a domain.Account) *aegisv1.Account {
	out := &aegisv1.Account{
		Id:            a.ID(),
		RealmId:       a.RealmID(),
		Type:          string(a.AccountType()),
		Status:        string(a.Status()),
		Email:         a.Email(),
		EmailVerified: a.EmailVerified(),
		DisplayName:   a.DisplayName(),
		CreatedAt:     timestamppb.New(a.CreatedAt()),
	}
	if t := a.LastLoginAt(); t != nil {
		out.LastLoginAt = timestamppb.New(*t)
	}
	return out
}
