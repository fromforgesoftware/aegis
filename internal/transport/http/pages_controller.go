package http

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"

	"github.com/fromforgesoftware/aegis/internal/app"
	"github.com/fromforgesoftware/aegis/internal/domain"
)

//go:embed templates/*.html
var templatesFS embed.FS

// PagesController serves the hosted auth UI at /auth/{type} via the kit's NewPageHandler.
type PagesController struct {
	flows    app.FlowUsecase
	oauth    app.OAuthUsecase
	realms   app.RealmUsecase
	renderer *kitrest.PageRenderer
	rl       kitrest.Middleware
}

func NewPagesController(flows app.FlowUsecase, oauth app.OAuthUsecase, realms app.RealmUsecase, rl *kitrest.RateLimitMiddleware) kitrest.Controller {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	return &PagesController{flows: flows, oauth: oauth, realms: realms, renderer: kitrest.NewPageRenderer(tmpl), rl: rl}
}

func (c *PagesController) Routes(r kitrest.Router) {
	r.Get("/auth/{type}", kitrest.NewPageHandler(c.renderer, c.decodeStart, c.start))
	r.With(c.rl).Post("/auth/{type}", kitrest.NewPageHandler(c.renderer, c.decodeSubmit, c.submit))
}

type flowView struct {
	Title    string
	Action   string
	Submit   string
	FlowID   string
	ReturnTo string
	Message  string
	Fields   []viewField
}

type viewField struct {
	Name      string
	Label     string
	InputType string
	Required  bool
}

type startInput struct {
	Segment  string
	FlowType domain.FlowType
	Realm    string
	ReturnTo string
}

type submitInput struct {
	Segment  string
	FlowType domain.FlowType
	FlowID   string
	ReturnTo string
	Payload  map[string]string
	Secure   bool
}

func (c *PagesController) decodeStart(_ context.Context, r *http.Request) (any, error) {
	seg := r.PathValue("type")
	ft, ok := flowTypeFromSegment(seg)
	if !ok {
		return nil, apierrors.NotFound("flow type", seg)
	}
	return startInput{
		Segment:  seg,
		FlowType: ft,
		Realm:    r.URL.Query().Get("realm"),
		ReturnTo: safeReturnTo(r.URL.Query().Get("return_to")),
	}, nil
}

func (c *PagesController) decodeSubmit(_ context.Context, r *http.Request) (any, error) {
	seg := r.PathValue("type")
	ft, ok := flowTypeFromSegment(seg)
	if !ok {
		return nil, apierrors.NotFound("flow type", seg)
	}
	if err := r.ParseForm(); err != nil {
		return nil, apierrors.InvalidArgument("malformed form body")
	}
	payload := map[string]string{}
	for _, f := range domain.RequiredFields(ft) {
		if v := r.FormValue(f.Name); v != "" {
			payload[f.Name] = v
		}
	}
	return submitInput{
		Segment:  seg,
		FlowType: ft,
		FlowID:   r.FormValue("flow"),
		ReturnTo: safeReturnTo(r.FormValue("return_to")),
		Payload:  payload,
		Secure:   requestScheme(r) == "https",
	}, nil
}

func (c *PagesController) start(ctx context.Context, in startInput) (kitrest.PageResult, error) {
	realm, err := c.realms.Get(ctx, app.RealmByName(in.Realm))
	if err != nil {
		return c.flowFormResult(in.Segment, in.FlowType, "", in.ReturnTo, "Unknown realm.", http.StatusNotFound), nil
	}
	f, err := c.flows.Create(ctx, domain.NewFlow(realm.ID(), in.FlowType, time.Time{}))
	if err != nil {
		return c.flowFormResult(in.Segment, in.FlowType, "", in.ReturnTo, userMessage(err), errStatus(err)), nil
	}
	return c.flowFormResult(in.Segment, in.FlowType, f.ID(), in.ReturnTo, "", http.StatusOK), nil
}

func (c *PagesController) submit(ctx context.Context, in submitInput) (kitrest.PageResult, error) {
	f, err := c.flows.Submit(ctx, app.SubmitFlowInput{FlowID: in.FlowID, Payload: in.Payload})
	if err != nil {
		return c.flowFormResult(in.Segment, in.FlowType, in.FlowID, in.ReturnTo, userMessage(err), errStatus(err)), nil
	}

	if in.FlowType == domain.FlowTypeLogin && f.ResultAccountID() != "" {
		sid, serr := c.oauth.StartSession(ctx, f.RealmID(), f.ResultAccountID())
		if serr != nil {
			return c.flowFormResult(in.Segment, in.FlowType, "", in.ReturnTo,
				"Something went wrong. Please try again.", http.StatusInternalServerError), nil
		}
		cookie := &http.Cookie{
			Name: sessionCookie, Value: sid, Path: "/",
			HttpOnly: true, Secure: in.Secure, SameSite: http.SameSiteLaxMode,
		}
		if in.ReturnTo != "" {
			return kitrest.PageResult{RedirectTo: in.ReturnTo, Cookies: []*http.Cookie{cookie}}, nil
		}
		return c.doneResult(in.FlowType, cookie), nil
	}
	return c.doneResult(in.FlowType, nil), nil
}

func (c *PagesController) flowFormResult(seg string, ft domain.FlowType, flowID, returnTo, message string, status int) kitrest.PageResult {
	title, submit, _ := pageMeta(ft)
	return kitrest.PageResult{
		Template: "flow.html",
		Status:   status,
		Data: flowView{
			Title: title, Action: "/auth/" + seg, Submit: submit,
			FlowID: flowID, ReturnTo: returnTo, Message: message, Fields: viewFields(ft),
		},
	}
}

func (c *PagesController) doneResult(ft domain.FlowType, cookie *http.Cookie) kitrest.PageResult {
	title, _, doneMsg := pageMeta(ft)
	res := kitrest.PageResult{Template: "done.html", Data: flowView{Title: title, Message: doneMsg}}
	if cookie != nil {
		res.Cookies = []*http.Cookie{cookie}
	}
	return res
}

// safeReturnTo permits only same-origin relative redirects, blocking open-redirect via return_to.
func safeReturnTo(rt string) string {
	if strings.HasPrefix(rt, "/") && !strings.HasPrefix(rt, "//") {
		return rt
	}
	return ""
}

func errStatus(err error) int {
	if s := apierrors.GetHTTPStatus(err); s != 0 {
		return s
	}
	return http.StatusInternalServerError
}

func flowTypeFromSegment(seg string) (domain.FlowType, bool) {
	switch seg {
	case "login":
		return domain.FlowTypeLogin, true
	case "registration":
		return domain.FlowTypeRegistration, true
	case "recovery":
		return domain.FlowTypeRecovery, true
	case "verification":
		return domain.FlowTypeVerification, true
	}
	return "", false
}

func pageMeta(t domain.FlowType) (title, submit, done string) {
	switch t {
	case domain.FlowTypeLogin:
		return "Sign in", "Sign in", "You are signed in."
	case domain.FlowTypeRegistration:
		return "Create account", "Create account", "Your account has been created."
	case domain.FlowTypeRecovery:
		return "Reset your password", "Send reset link", "If that email is registered, a reset link is on its way."
	case domain.FlowTypeVerification:
		return "Verify your email", "Verify", "Your email has been verified."
	}
	return "", "", ""
}

func viewFields(t domain.FlowType) []viewField {
	var out []viewField
	for _, f := range domain.RequiredFields(t) {
		out = append(out, viewField{Name: f.Name, Label: fieldLabel(f.Name), InputType: inputType(f.Kind), Required: f.Required})
	}
	return out
}

func fieldLabel(name string) string {
	switch name {
	case "email":
		return "Email"
	case "password":
		return "Password"
	case "displayName":
		return "Display name"
	case "token":
		return "Verification code"
	}
	return name
}

func inputType(kind string) string {
	switch kind {
	case "email":
		return "email"
	case "password":
		return "password"
	}
	return "text"
}

// userMessage masks 5xx internals while passing client-error messages through.
func userMessage(err error) string {
	if apierrors.GetHTTPStatus(err) >= http.StatusInternalServerError {
		return "Something went wrong. Please try again."
	}
	return err.Error()
}
