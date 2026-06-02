package http

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/api"
	"github.com/fromforgesoftware/aegis/internal/app"
)

// AccountModerationController exposes ban/unban/merge over the account root.
// It's the first admin surface over aegis.account — full account CRUD is a
// later wave.
type AccountModerationController struct {
	moderation app.AccountModerationUsecase
	merge      app.AccountMergeUsecase
}

func NewAccountModerationController(moderation app.AccountModerationUsecase, merge app.AccountMergeUsecase) kitrest.Controller {
	return &AccountModerationController{moderation: moderation, merge: merge}
}

func (c *AccountModerationController) Routes(r kitrest.Router) {
	r.Route("/api/accounts", func(r kitrest.Router) {
		r.Post("/ban", kitrest.NewJsonApiCommandHandler(
			c.ban, decodeBan, identityAccountBanDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Ban an account"), openapi.Tags("moderation"), openapi.Errors(400, 404)),
		))
		r.Post("/unban", kitrest.NewJsonApiCommandHandler(
			c.unban, decodeUnban, identityAccountBanDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Unban an account"), openapi.Tags("moderation"), openapi.Errors(400)),
		))
		r.Post("/merge", kitrest.NewJsonApiCommandHandler(
			c.mergeAccounts, decodeMerge, identityAccountMergeDTO,
			kitrest.HandlerWithSuccessStatus(http.StatusOK),
			kitrest.HandlerWithOpenAPI(openapi.Summary("Merge a duplicate account into a survivor"), openapi.Tags("moderation"), openapi.Errors(400)),
		))
	})
}

func (c *AccountModerationController) mergeAccounts(ctx context.Context, in api.MergeRequestDTO) (*api.AccountMergeDTO, error) {
	summary, err := c.merge.Merge(ctx, in.RSourceID, in.RTargetID)
	if err != nil {
		return nil, err
	}
	return api.AccountMergeToDTO(in.RSourceID, in.RTargetID, summary.ExternalIDs, summary.Memberships, summary.Bindings), nil
}

func decodeMerge(req *http.Request) (api.MergeRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.MergeRequestDTO](req)
	if err != nil {
		return api.MergeRequestDTO{}, err
	}
	return *body, nil
}

func identityAccountMergeDTO(dto *api.AccountMergeDTO) *api.AccountMergeDTO { return dto }

func (c *AccountModerationController) ban(ctx context.Context, in api.BanRequestDTO) (*api.AccountBanDTO, error) {
	if err := c.moderation.Ban(ctx, in.RAccountID, in.RUntil, in.RReason); err != nil {
		return nil, err
	}
	return api.AccountBanToDTO(in.RAccountID, true, in.RUntil, in.RReason), nil
}

func (c *AccountModerationController) unban(ctx context.Context, in api.UnbanRequestDTO) (*api.AccountBanDTO, error) {
	if err := c.moderation.Unban(ctx, in.RAccountID); err != nil {
		return nil, err
	}
	return api.AccountBanToDTO(in.RAccountID, false, nil, ""), nil
}

func decodeBan(req *http.Request) (api.BanRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.BanRequestDTO](req)
	if err != nil {
		return api.BanRequestDTO{}, err
	}
	return *body, nil
}

func decodeUnban(req *http.Request) (api.UnbanRequestDTO, error) {
	body, err := kitrest.UnmarshalPayloadFromRequest[*api.UnbanRequestDTO](req)
	if err != nil {
		return api.UnbanRequestDTO{}, err
	}
	return *body, nil
}

func identityAccountBanDTO(dto *api.AccountBanDTO) *api.AccountBanDTO { return dto }
