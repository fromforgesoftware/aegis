package grpc

import (
	"context"

	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"

	"github.com/fromforgesoftware/aegis/internal/app"
	aegisv1 "github.com/fromforgesoftware/aegis/pkg/api/aegis/v1"
)

type identityBrokerController struct {
	broker app.IdentityBrokerUsecase
}

// NewIdentityBrokerController exposes ResolveAccount over gRPC for hot-path
// S2S federation callers.
func NewIdentityBrokerController(broker app.IdentityBrokerUsecase) kitgrpc.Controller {
	return &identityBrokerController{broker: broker}
}

func (c *identityBrokerController) SD() kitgrpc.ServiceDesc {
	return &aegisv1.IdentityBrokerService_ServiceDesc
}

func (c *identityBrokerController) ResolveAccount(ctx context.Context, req *aegisv1.ResolveAccountRequest) (*aegisv1.ResolveAccountResponse, error) {
	res, err := c.broker.ResolveAccount(ctx, app.ResolveAccountInput{
		RealmID:  req.GetRealmId(),
		IDPName:  req.GetIdpName(),
		RawToken: req.GetToken(),
	})
	if err != nil {
		return nil, err
	}
	resp := &aegisv1.ResolveAccountResponse{
		Created:      res.Created,
		LinkRequired: res.LinkRequired,
	}
	if res.Account != nil {
		resp.AccountId = res.Account.ID()
		resp.Email = res.Account.Email()
		resp.DisplayName = res.Account.DisplayName()
	}
	return resp, nil
}
