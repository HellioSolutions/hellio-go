// Package hellio is the official Go client for the Hellio Messaging API v1:
// SMS, OTP (SMS / voice / WhatsApp / email), voice broadcasts, number lookup (HLR), email
// verification, USSD, and webhooks.
//
// Create a client with a Bearer token and call one method per endpoint. Most
// calls return the decoded JSON response as a map[string]any (payloads live under
// the "data" key); the USSD methods on client.USSD decode into typed structs
// instead. Non-2xx responses return a typed *Error.
package hellio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production API root.
	DefaultBaseURL = "https://api.helliomessaging.com/v1"
	// DefaultTimeout is the per-request timeout when none is configured.
	DefaultTimeout = 30 * time.Second
)

// Client talks to the Hellio Messaging API. Create one with NewClient and reuse
// it; it is safe for concurrent use.
type Client struct {
	token         string
	baseURL       string
	defaultSender string
	http          *http.Client

	// USSD groups the USSD endpoints (pricing, apps, extensions, sessions,
	// simulate). See ussd.go.
	USSD *USSDService
}

// Option configures a Client. Pass options to NewClient.
type Option func(*Client)

// WithBaseURL overrides the API root (default DefaultBaseURL).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

// WithTimeout sets the per-request timeout on the default HTTP client. It has no
// effect if you also pass WithHTTPClient; set the timeout on that client instead.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// WithDefaultSender sets the Sender ID used by SMS when no sender is given.
func WithDefaultSender(sender string) Option {
	return func(c *Client) { c.defaultSender = sender }
}

// WithHTTPClient injects a custom *http.Client. Handy for tests and for tuning
// transport, proxies, or timeouts.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}

// NewClient builds a client. The token falls back to HELLIO_API_TOKEN, the base
// URL to HELLIO_BASE_URL, and the default sender to HELLIO_DEFAULT_SENDER when
// those are left empty.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		token:         token,
		baseURL:       DefaultBaseURL,
		defaultSender: os.Getenv("HELLIO_DEFAULT_SENDER"),
		http:          &http.Client{Timeout: DefaultTimeout},
	}
	if c.token == "" {
		c.token = os.Getenv("HELLIO_API_TOKEN")
	}
	if v := os.Getenv("HELLIO_BASE_URL"); v != "" {
		c.baseURL = v
	}
	for _, o := range opts {
		o(c)
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	c.USSD = &USSDService{client: c}
	return c
}

// ---------------------------------------------------------------- Account

// Balance returns the account balance. GET balance.
func (c *Client) Balance(ctx context.Context) (map[string]any, error) {
	return c.get(ctx, "balance", nil)
}

// Pricing returns per-network SMS pricing. Pass an ISO-2 country code to narrow
// the result, or "" for all. GET pricing.
func (c *Client) Pricing(ctx context.Context, country string) (map[string]any, error) {
	var q url.Values
	if country != "" {
		q = url.Values{"country": {country}}
	}
	return c.get(ctx, "pricing", q)
}

// ---------------------------------------------------------------- SMS

// SMS sends a text message. recipients may be single numbers or comma-separated
// strings; they are flattened into a list. Pass sender "" to use the configured
// default sender, and gateway "" to omit it. POST sms/send.
func (c *Client) SMS(ctx context.Context, recipients []string, message, sender, gateway string) (map[string]any, error) {
	if sender == "" {
		sender = c.defaultSender
	}
	body := map[string]any{
		"recipients": Recipients(recipients...),
		"message":    message,
	}
	setStr(body, "sender", sender)
	setStr(body, "gateway", gateway)
	return c.post(ctx, "sms/send", body)
}

// Message returns the delivery status of a single message. GET messages/{id}.
func (c *Client) Message(ctx context.Context, id int) (map[string]any, error) {
	return c.get(ctx, "messages/"+strconv.Itoa(id), nil)
}

// Campaign returns a campaign summary. GET campaigns/{id}.
func (c *Client) Campaign(ctx context.Context, id int) (map[string]any, error) {
	return c.get(ctx, "campaigns/"+strconv.Itoa(id), nil)
}

// ---------------------------------------------------------------- OTP

// OTP sends a one-time passcode. "to" is a phone number (sms/voice/whatsapp) or an
// email (email). sender (a Sender ID) is required for sms/voice and ignored for
// whatsapp and email. channel defaults to "sms" when empty (sms/voice/whatsapp/email).
// Pass length or expiry as 0 to omit them,
// and purpose or gateway as "" to omit them. POST otp/send.
func (c *Client) OTP(ctx context.Context, to, sender, channel, purpose string, length, expiry int, gateway string) (map[string]any, error) {
	if channel == "" {
		channel = "sms"
	}
	body := map[string]any{"channel": channel}
	body[toField(channel)] = to
	setStr(body, "sender", sender)
	setStr(body, "purpose", purpose)
	setInt(body, "length", length)
	setInt(body, "expiry", expiry)
	setStr(body, "gateway", gateway)
	return c.post(ctx, "otp/send", body)
}

// VerifyOTP checks a passcode and returns the full response. channel defaults to
// "sms" when empty. POST otp/verify.
func (c *Client) VerifyOTP(ctx context.Context, to, code, channel string) (map[string]any, error) {
	if channel == "" {
		channel = "sms"
	}
	body := map[string]any{"channel": channel, "code": code}
	body[toField(channel)] = to
	return c.post(ctx, "otp/verify", body)
}

// Verify is a convenience wrapper that returns true when the code is valid. A 422
// validation error is treated as "not verified" and returns (false, nil); other
// errors are returned as-is.
func (c *Client) Verify(ctx context.Context, to, code, channel string) (bool, error) {
	res, err := c.VerifyOTP(ctx, to, code, channel)
	if err != nil {
		if e, ok := err.(*Error); ok && e.Kind == KindValidation {
			return false, nil
		}
		return false, err
	}
	if data, ok := res["data"].(map[string]any); ok {
		return truthy(data["verified"]), nil
	}
	return false, nil
}

// ---------------------------------------------------------------- Voice

// Voice starts a voice broadcast. Provide text (read out with TTS) or audioURL (a
// hosted audio file); pass "" for the one you do not use. name is an optional
// label. POST voice/send.
func (c *Client) Voice(ctx context.Context, recipients []string, callerID, text, audioURL, name string) (map[string]any, error) {
	body := map[string]any{
		"recipients": Recipients(recipients...),
		"caller_id":  callerID,
	}
	setStr(body, "text", text)
	setStr(body, "audio_url", audioURL)
	setStr(body, "name", name)
	return c.post(ctx, "voice/send", body)
}

// VoiceStatus returns the status of a voice broadcast. GET voice/{id}.
func (c *Client) VoiceStatus(ctx context.Context, id int) (map[string]any, error) {
	return c.get(ctx, "voice/"+strconv.Itoa(id), nil)
}

// ---------------------------------------------------------------- Number lookup

// Lookup queues an HLR lookup for the given numbers (async; poll the result with
// LookupResult). POST lookup.
func (c *Client) Lookup(ctx context.Context, numbers []string) (map[string]any, error) {
	return c.post(ctx, "lookup", map[string]any{"numbers": Recipients(numbers...)})
}

// Lookups lists past lookups. GET lookups.
func (c *Client) Lookups(ctx context.Context) (map[string]any, error) {
	return c.get(ctx, "lookups", nil)
}

// LookupResult returns a single lookup result. GET lookup/{id}.
func (c *Client) LookupResult(ctx context.Context, id int) (map[string]any, error) {
	return c.get(ctx, "lookup/"+strconv.Itoa(id), nil)
}

// ---------------------------------------------------------------- Email verify

// VerifyEmail validates a list of email addresses. POST email/verify.
func (c *Client) VerifyEmail(ctx context.Context, emails []string) (map[string]any, error) {
	return c.post(ctx, "email/verify", map[string]any{"emails": Recipients(emails...)})
}

// ---------------------------------------------------------------- Webhooks

// Webhooks lists configured webhooks. GET webhooks.
func (c *Client) Webhooks(ctx context.Context) (map[string]any, error) {
	return c.get(ctx, "webhooks", nil)
}

// CreateWebhook registers a webhook URL for the given events (empty events is
// omitted from the request). POST webhooks.
func (c *Client) CreateWebhook(ctx context.Context, webhookURL string, events []string) (map[string]any, error) {
	body := map[string]any{"url": webhookURL}
	if len(events) > 0 {
		body["events"] = events
	}
	return c.post(ctx, "webhooks", body)
}

// DeleteWebhook removes a webhook. DELETE webhooks/{id}.
func (c *Client) DeleteWebhook(ctx context.Context, id int) (map[string]any, error) {
	return c.request(ctx, http.MethodDelete, "webhooks/"+strconv.Itoa(id), nil, nil)
}

// ---------------------------------------------------------------- internals

func (c *Client) get(ctx context.Context, path string, query url.Values) (map[string]any, error) {
	return c.request(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) post(ctx context.Context, path string, body map[string]any) (map[string]any, error) {
	return c.request(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body map[string]any) (map[string]any, error) {
	target := c.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	data := map[string]any{}
	if len(raw) > 0 {
		// Ignore decode errors; a non-JSON body simply yields an empty map.
		_ = json.Unmarshal(raw, &data)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, nil
	}

	message := "Hellio API request failed."
	if m, ok := data["message"].(string); ok && m != "" {
		message = m
	}
	return nil, newError(resp.StatusCode, message, data)
}

// Recipients flattens single numbers and comma-separated strings into a trimmed,
// non-empty list. hellio.Recipients("233...", "233...,244...") is valid.
func Recipients(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func toField(channel string) string {
	if channel == "email" {
		return "email"
	}
	return "mobile_number"
}

func setStr(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	}
}

func setInt(m map[string]any, key string, val int) {
	if val != 0 {
		m[key] = val
	}
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t != "" && t != "false" && t != "0"
	case float64:
		return t != 0
	case nil:
		return false
	default:
		return true
	}
}
