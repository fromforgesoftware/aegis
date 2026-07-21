package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/google/uuid"
)

const jsonAPIMediaType = "application/vnd.api+json"

// gatewayTokenIssuer mints the shared-secret bearer the Aegis gateway expects
// on /api/* when FORGE_GATEWAY_SECRET is set. Satisfied by go-kit's HMAC
// issuer.
type gatewayTokenIssuer interface {
	Issue(ctx context.Context, accountID uuid.UUID, username string) (string, error)
}

// restClient is the shared HTTP transport under every JSON:API surface: it
// owns the base URL, the tuned http.Client, gateway-bearer minting and the
// JSON:API error → kit error mapping.
type restClient struct {
	base    string
	http    *http.Client
	issuer  gatewayTokenIssuer
	svcID   uuid.UUID
	svcName string
}

func newRESTClient(baseURL string, issuer gatewayTokenIssuer, serviceName string) *restClient {
	httpClient := kitrest.NewDefaultHTTPClient()
	return &restClient{
		base:    strings.TrimRight(baseURL, "/"),
		http:    httpClient,
		issuer:  issuer,
		svcID:   uuid.New(),
		svcName: serviceName,
	}
}

// do performs a request, attaching the gateway bearer when configured, and
// returns the response when 2xx or the decoded API error otherwise.
func (c *restClient) do(ctx context.Context, method, path, contentType string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", jsonAPIMediaType)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	if c.issuer != nil {
		token, err := c.issuer.Issue(ctx, c.svcID, c.svcName)
		if err != nil {
			return nil, fmt.Errorf("aegis client: mint gateway token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		return nil, decodeRESTError(method, path, resp)
	}
	return resp, nil
}

// decodeRESTError maps a JSON:API error document back to its kit error code;
// non-JSON:API bodies degrade to a generic error carrying the HTTP status.
func decodeRESTError(method, path string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if errorObjects, err := jsonapi.UnmarshalErrors(bytes.NewReader(body)); err == nil && len(errorObjects) > 0 {
		first := errorObjects[0]
		message := first.Detail
		if message == "" {
			message = first.Title
		}
		return apierrors.New(apierrors.Code(first.Code),
			apierrors.WithMessage(message),
			apierrors.WithHTTPStatus(resp.StatusCode),
		)
	}
	return apierrors.New(apierrors.CodeInternalError,
		apierrors.WithMessage(fmt.Sprintf("aegis client: %s %s: %s: %s",
			method, path, resp.Status, strings.TrimSpace(string(body)))),
		apierrors.WithHTTPStatus(resp.StatusCode),
	)
}

// restCreate POSTs one JSON:API resource and decodes the created resource.
func restCreate[T any](ctx context.Context, c *restClient, path string, dto any) (T, error) {
	var zero T
	var buf bytes.Buffer
	if err := jsonapi.MarshalPayload(&buf, dto); err != nil {
		return zero, err
	}
	resp, err := c.do(ctx, http.MethodPost, path, jsonAPIMediaType, buf.Bytes())
	if err != nil {
		return zero, err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := jsonapi.UnmarshalPayload[T](resp.Body)
	if err != nil {
		return zero, err
	}
	return result.Data, nil
}

// restGet GETs one JSON:API resource by path.
func restGet[T any](ctx context.Context, c *restClient, path string) (T, error) {
	var zero T
	resp, err := c.do(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return zero, err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := jsonapi.UnmarshalPayload[T](resp.Body)
	if err != nil {
		return zero, err
	}
	return result.Data, nil
}

// restList GETs a JSON:API collection, applying the list options as query
// parameters.
func restList[T any](ctx context.Context, c *restClient, path string, opts ...ListOption) ([]T, error) {
	values := url.Values{}
	for _, opt := range opts {
		opt(values)
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := c.do(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := jsonapi.UnmarshalManyPayload[T](resp.Body)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// restDelete DELETEs a resource by path.
func restDelete(ctx context.Context, c *restClient, path string) error {
	resp, err := c.do(ctx, http.MethodDelete, path, "", nil)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// restJSON exchanges plain-JSON documents (relationship endpoints and action
// endpoints that don't speak vnd.api+json). in and out may be nil.
func restJSON(ctx context.Context, c *restClient, method, path string, in, out any) error {
	var body []byte
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = encoded
	}
	resp, err := c.do(ctx, method, path, "application/json", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// identifierDocument is the {data:[{type,id}]} shape relationship endpoints
// exchange.
type identifierDocument struct {
	Data []identifierRef `json:"data"`
}

type identifierRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func identifierDocumentOf(resourceType string, ids []string) identifierDocument {
	refs := make([]identifierRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, identifierRef{Type: resourceType, ID: id})
	}
	return identifierDocument{Data: refs}
}

func (d identifierDocument) ids() []string {
	ids := make([]string, 0, len(d.Data))
	for _, ref := range d.Data {
		ids = append(ids, ref.ID)
	}
	return ids
}

// ListOption narrows or pages a JSON:API list call.
type ListOption func(url.Values)

// WithFilter adds filter[field][operator]=value (e.g. "realmId", "eq", id).
func WithFilter(field, operator, value string) ListOption {
	return func(values url.Values) {
		values.Set(fmt.Sprintf("filter[%s][%s]", field, operator), value)
	}
}

// WithPageLimit caps the number of results.
func WithPageLimit(limit int) ListOption {
	return func(values url.Values) {
		values.Set("page[limit]", strconv.Itoa(limit))
	}
}

// WithPageOffset skips the first n results.
func WithPageOffset(offset int) ListOption {
	return func(values url.Values) {
		values.Set("page[offset]", strconv.Itoa(offset))
	}
}

// WithSort orders results ("name", "-createdAt").
func WithSort(fields ...string) ListOption {
	return func(values url.Values) {
		values.Set("sort", strings.Join(fields, ","))
	}
}
