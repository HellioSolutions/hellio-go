package hellio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// USSDService groups the USSD endpoints. Reach it through the client as
// client.USSD, e.g. client.USSD.Pricing(ctx) or client.USSD.Apps(ctx, "").
//
// Unlike the other client methods (which return map[string]any), the USSD
// methods decode the "data" payload into typed structs. Non-2xx responses still
// return a typed *Error: a rented extension that is gone returns KindConflict
// (409, body error "extension_unavailable"), and a top-up shortfall returns
// KindInsufficientBalance (402, body error "insufficient_balance").
type USSDService struct {
	client *Client
}

// ---------------------------------------------------------------- types

// USSDApp is a registered USSD application. Hellio POSTs inbound session events
// to CallbackURL, signed with Secret (X-Hellio-Signature: HMAC-SHA256 of the raw
// body). Keep Secret private; it is returned so you can verify signatures.
type USSDApp struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	CallbackURL string `json:"callback_url"`
	Secret      string `json:"secret"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
}

// USSDAppInput is the body for creating or updating an app. Active is a pointer
// so it can be omitted on create (the server sets a default) and set explicitly
// on update; pass Active with a &true / &false value to change it.
type USSDAppInput struct {
	Name        string `json:"name"`
	CallbackURL string `json:"callback_url"`
	Active      *bool  `json:"active,omitempty"`
}

// USSDExtension is a rented dial-code extension bound to an app. AppID is nil
// while the extension is unassigned.
type USSDExtension struct {
	ID           int    `json:"id"`
	Code         string `json:"code"`
	DialString   string `json:"dial_string"`
	Length       int    `json:"length"`
	Status       string `json:"status"`
	MonthlyPrice string `json:"monthly_price"`
	AutoRenew    bool   `json:"auto_renew"`
	AppID        *int   `json:"app_id"`
	ExpiresAt    string `json:"expires_at"`
}

// USSDSession is a single USSD dialog. Sandbox is true for simulated sessions.
type USSDSession struct {
	ID          int    `json:"id"`
	SessionRef  string `json:"session_ref"`
	Msisdn      string `json:"msisdn"`
	ServiceCode string `json:"service_code"`
	Status      string `json:"status"`
	Steps       int    `json:"steps"`
	Charge      string `json:"charge"`
	Sandbox     bool   `json:"sandbox"`
	StartedAt   string `json:"started_at"`
	EndedAt     string `json:"ended_at"`
}

// USSDPricing is the current USSD price sheet.
type USSDPricing struct {
	ShortCode       string               `json:"short_code"`
	Currency        string               `json:"currency"`
	SessionPrices   []USSDSessionPrice   `json:"session_prices"`
	ExtensionPrices []USSDExtensionPrice `json:"extension_prices"`
}

// USSDSessionPrice is the per-session charge for one network.
type USSDSessionPrice struct {
	Network      string `json:"network"`
	Slug         string `json:"slug"`
	SessionPrice string `json:"session_price"`
}

// USSDExtensionPrice is the monthly rent for an extension of a given length.
type USSDExtensionPrice struct {
	Length       int    `json:"length"`
	MonthlyPrice string `json:"monthly_price"`
}

// USSDAvailability reports whether a candidate extension code can be rented.
// MonthlyPrice is nil when the code is not valid or not available.
type USSDAvailability struct {
	Code         string  `json:"code"`
	Valid        bool    `json:"valid"`
	Available    bool    `json:"available"`
	MonthlyPrice *string `json:"monthly_price"`
}

// USSDSimulateRequest drives Simulate. Leave SessionID empty and set NewSession
// true to open a fresh dialog; on later steps pass the SessionID returned by the
// first call and NewSession false. Input carries the subscriber's latest entry.
type USSDSimulateRequest struct {
	SessionID   string `json:"session_id,omitempty"`
	Msisdn      string `json:"msisdn"`
	ServiceCode string `json:"service_code"`
	Input       string `json:"input"`
	NewSession  bool   `json:"new_session"`
}

// USSDSimulateResult is the app's reply for one simulated step. Action is
// "continue" (prompt again) or "end" (final screen); Continue mirrors it as a
// bool for convenience.
type USSDSimulateResult struct {
	Message  string `json:"message"`
	Action   string `json:"action"`
	Continue bool   `json:"continue"`
}

// ---------------------------------------------------------------- pricing

// Pricing returns the USSD price sheet (session prices per network and monthly
// extension rents). GET ussd/pricing.
func (s *USSDService) Pricing(ctx context.Context) (*USSDPricing, error) {
	var out USSDPricing
	if _, err := s.call(ctx, http.MethodGet, "ussd/pricing", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Availability checks whether a candidate extension code can be rented and at
// what monthly price. GET ussd/pricing/availability?code=...
func (s *USSDService) Availability(ctx context.Context, code string) (*USSDAvailability, error) {
	var out USSDAvailability
	q := url.Values{"code": {code}}
	if _, err := s.call(ctx, http.MethodGet, "ussd/pricing/availability", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------- apps

// Apps lists your USSD apps. Pass "" for the first page; the returned nextCursor
// is "" once there are no more pages. GET ussd/apps.
func (s *USSDService) Apps(ctx context.Context, cursor string) (apps []USSDApp, nextCursor string, err error) {
	nextCursor, err = s.call(ctx, http.MethodGet, "ussd/apps", cursorQuery(cursor), nil, &apps)
	return apps, nextCursor, err
}

// CreateApp registers a new app. Name and CallbackURL are required; Active is
// ignored on create. POST ussd/apps.
func (s *USSDService) CreateApp(ctx context.Context, input USSDAppInput) (*USSDApp, error) {
	input.Active = nil
	var out USSDApp
	if _, err := s.call(ctx, http.MethodPost, "ussd/apps", nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateApp updates an existing app. Set input.Active (with a &true / &false) to
// enable or disable inbound delivery. PUT ussd/apps/{id}.
func (s *USSDService) UpdateApp(ctx context.Context, id int, input USSDAppInput) (*USSDApp, error) {
	var out USSDApp
	if _, err := s.call(ctx, http.MethodPut, "ussd/apps/"+strconv.Itoa(id), nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteApp removes an app. DELETE ussd/apps/{id}.
func (s *USSDService) DeleteApp(ctx context.Context, id int) error {
	_, err := s.call(ctx, http.MethodDelete, "ussd/apps/"+strconv.Itoa(id), nil, nil, nil)
	return err
}

// ---------------------------------------------------------------- extensions

// Extensions lists your rented extensions. Pass "" for the first page; the
// returned nextCursor is "" once there are no more pages. GET ussd/extensions.
func (s *USSDService) Extensions(ctx context.Context, cursor string) (extensions []USSDExtension, nextCursor string, err error) {
	nextCursor, err = s.call(ctx, http.MethodGet, "ussd/extensions", cursorQuery(cursor), nil, &extensions)
	return extensions, nextCursor, err
}

// RentExtension rents an extension code, optionally binding it to an app (pass
// appID 0 to leave it unassigned). Returns KindConflict (409,
// "extension_unavailable") if the code was taken, or KindInsufficientBalance
// (402, "insufficient_balance") if the account cannot cover the rent.
// POST ussd/extensions.
func (s *USSDService) RentExtension(ctx context.Context, code string, appID int) (*USSDExtension, error) {
	body := map[string]any{"code": code}
	if appID != 0 {
		body["app_id"] = appID
	}
	var out USSDExtension
	if _, err := s.call(ctx, http.MethodPost, "ussd/extensions", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReleaseExtension gives up a rented extension. DELETE ussd/extensions/{id}.
func (s *USSDService) ReleaseExtension(ctx context.Context, id int) error {
	_, err := s.call(ctx, http.MethodDelete, "ussd/extensions/"+strconv.Itoa(id), nil, nil, nil)
	return err
}

// ---------------------------------------------------------------- sessions

// Sessions lists USSD sessions. Pass status "" for all or a filter such as
// "ended", and cursor "" for the first page. GET ussd/sessions.
func (s *USSDService) Sessions(ctx context.Context, status, cursor string) (sessions []USSDSession, nextCursor string, err error) {
	q := cursorQuery(cursor)
	if status != "" {
		if q == nil {
			q = url.Values{}
		}
		q.Set("status", status)
	}
	nextCursor, err = s.call(ctx, http.MethodGet, "ussd/sessions", q, nil, &sessions)
	return sessions, nextCursor, err
}

// Session returns a single session by id. GET ussd/sessions/{id}.
func (s *USSDService) Session(ctx context.Context, id int) (*USSDSession, error) {
	var out USSDSession
	if _, err := s.call(ctx, http.MethodGet, "ussd/sessions/"+strconv.Itoa(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------- simulate

// Simulate runs one step of a USSD dialog against your app's callback without a
// real subscriber, so you can test flows end to end. POST ussd/simulate.
func (s *USSDService) Simulate(ctx context.Context, req USSDSimulateRequest) (*USSDSimulateResult, error) {
	var out USSDSimulateResult
	if _, err := s.call(ctx, http.MethodPost, "ussd/simulate", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------- internals

// call performs a request and decodes the response envelope's "data" into out
// (when out is non-nil). It returns meta.next_cursor for paginated list
// endpoints ("" when absent) and a typed *Error for non-2xx responses.
func (s *USSDService) call(ctx context.Context, method, path string, query url.Values, body, out any) (string, error) {
	c := s.client
	target := c.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ussdError(resp.StatusCode, raw)
	}

	if len(raw) == 0 {
		return "", nil
	}

	var env struct {
		Data json.RawMessage `json:"data"`
		Meta struct {
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return "", err
		}
	}
	return env.Meta.NextCursor, nil
}

// ussdError builds a typed *Error from a non-2xx USSD response. The USSD
// endpoints report the reason under "error" (e.g. "extension_unavailable"),
// while the rest of the API uses "message"; both are honored, and the decoded
// body is preserved on Error.Body.
func ussdError(status int, raw []byte) *Error {
	body := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	message := "Hellio API request failed."
	if m, ok := body["message"].(string); ok && m != "" {
		message = m
	} else if e, ok := body["error"].(string); ok && e != "" {
		message = e
	}
	return newError(status, message, body)
}

// cursorQuery builds the pagination query for a list request, or nil for the
// first page.
func cursorQuery(cursor string) url.Values {
	if cursor == "" {
		return nil
	}
	return url.Values{"cursor": {cursor}}
}
