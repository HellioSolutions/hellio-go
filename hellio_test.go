package hellio

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// newTestClient returns a client pointed at a test server plus the recorded
// requests, so tests can assert method, path, and body.
type recorded struct {
	method string
	path   string
	query  string
	body   map[string]any
}

func newTestClient(t *testing.T, status int, response any, rec *recorded) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec != nil {
			rec.method = r.Method
			rec.path = r.URL.Path
			rec.query = r.URL.RawQuery
			raw, _ := io.ReadAll(r.Body)
			// Reset per request: json.Unmarshal merges into an existing map, so a
			// reused recorder would otherwise retain keys from an earlier call.
			rec.body = nil
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &rec.body)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %q, want Bearer test-token", got)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)
	return NewClient("test-token", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

func TestBalance(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"balance": "195.0000"}}, &rec)

	res, err := c.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance error: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/balance" {
		t.Errorf("request = %s %s, want GET /balance", rec.method, rec.path)
	}
	data, ok := res["data"].(map[string]any)
	if !ok || data["balance"] != "195.0000" {
		t.Errorf("unexpected response: %v", res)
	}
}

func TestPricingCountryQuery(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": []any{}}, &rec)

	if _, err := c.Pricing(context.Background(), "GH"); err != nil {
		t.Fatalf("Pricing error: %v", err)
	}
	if rec.query != "country=GH" {
		t.Errorf("query = %q, want country=GH", rec.query)
	}

	if _, err := c.Pricing(context.Background(), ""); err != nil {
		t.Fatalf("Pricing error: %v", err)
	}
	if rec.query != "" {
		t.Errorf("query = %q, want empty", rec.query)
	}
}

func TestSMS(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"status": "queued"}}, &rec)
	c.defaultSender = "HellioSMS"

	_, err := c.SMS(context.Background(), []string{"233241234567"}, "Hello!", "", "")
	if err != nil {
		t.Fatalf("SMS error: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/sms/send" {
		t.Errorf("request = %s %s, want POST /sms/send", rec.method, rec.path)
	}
	if rec.body["sender"] != "HellioSMS" {
		t.Errorf("sender = %v, want HellioSMS (default)", rec.body["sender"])
	}
	if rec.body["message"] != "Hello!" {
		t.Errorf("message = %v", rec.body["message"])
	}
	if _, ok := rec.body["gateway"]; ok {
		t.Errorf("gateway should be omitted when empty, got %v", rec.body["gateway"])
	}
	got := toStringSlice(rec.body["recipients"])
	if !reflect.DeepEqual(got, []string{"233241234567"}) {
		t.Errorf("recipients = %v", got)
	}
}

func TestSMSRecipientNormalization(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{}}, &rec)

	// Mixed single, comma-separated, and spaced entries collapse to a flat list.
	_, err := c.SMS(context.Background(), []string{"233241234567, 233201234567", "  233555000111  "}, "Hi", "SENDER", "")
	if err != nil {
		t.Fatalf("SMS error: %v", err)
	}
	got := toStringSlice(rec.body["recipients"])
	want := []string{"233241234567", "233201234567", "233555000111"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("recipients = %v, want %v", got, want)
	}
}

func TestRecipientsHelper(t *testing.T) {
	got := Recipients("a", "b,c", " d , ,e ")
	want := []string{"a", "b", "c", "d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Recipients = %v, want %v", got, want)
	}
}

func TestOTPChannels(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"status": "queued"}}, &rec)

	// SMS channel uses mobile_number and includes length/expiry.
	if _, err := c.OTP(context.Background(), "233241234567", "HellioSMS", "sms", "", 6, 10, ""); err != nil {
		t.Fatalf("OTP error: %v", err)
	}
	if rec.path != "/otp/send" || rec.body["channel"] != "sms" {
		t.Errorf("unexpected otp request: %s %v", rec.path, rec.body)
	}
	if rec.body["mobile_number"] != "233241234567" {
		t.Errorf("mobile_number = %v", rec.body["mobile_number"])
	}
	if rec.body["length"] != float64(6) || rec.body["expiry"] != float64(10) {
		t.Errorf("length/expiry = %v/%v", rec.body["length"], rec.body["expiry"])
	}

	// Email channel uses email and omits sender.
	if _, err := c.OTP(context.Background(), "user@example.com", "", "email", "", 0, 0, ""); err != nil {
		t.Fatalf("OTP email error: %v", err)
	}
	if rec.body["email"] != "user@example.com" {
		t.Errorf("email = %v", rec.body["email"])
	}
	if _, ok := rec.body["sender"]; ok {
		t.Errorf("sender should be omitted for email")
	}
	if _, ok := rec.body["length"]; ok {
		t.Errorf("length should be omitted when zero")
	}
}

func TestVerifyOTPDefaultChannel(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"verified": true}}, &rec)

	if _, err := c.VerifyOTP(context.Background(), "233241234567", "123456", ""); err != nil {
		t.Fatalf("VerifyOTP error: %v", err)
	}
	if rec.path != "/otp/verify" || rec.body["channel"] != "sms" {
		t.Errorf("unexpected request: %s %v", rec.path, rec.body)
	}
	if rec.body["code"] != "123456" {
		t.Errorf("code = %v", rec.body["code"])
	}
}

func TestVerifyTrue(t *testing.T) {
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"verified": true}}, nil)
	ok, err := c.Verify(context.Background(), "233241234567", "123456", "sms")
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !ok {
		t.Errorf("Verify = false, want true")
	}
}

func TestVerifyFalseOnValidationError(t *testing.T) {
	// A 422 means the code was wrong; Verify should return false, nil.
	c := newTestClient(t, 422, map[string]any{"message": "Invalid code", "errors": map[string]any{"code": []any{"invalid"}}}, nil)
	ok, err := c.Verify(context.Background(), "233241234567", "000000", "sms")
	if err != nil {
		t.Fatalf("Verify should swallow 422, got error: %v", err)
	}
	if ok {
		t.Errorf("Verify = true, want false")
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status   int
		sentinel *Error
		kind     Kind
	}{
		{401, ErrInvalidApiToken, KindInvalidApiToken},
		{402, ErrInsufficientBalance, KindInsufficientBalance},
		{422, ErrValidation, KindValidation},
		{429, ErrRateLimit, KindRateLimit},
		{500, nil, KindGeneric},
	}
	for _, tc := range cases {
		c := newTestClient(t, tc.status, map[string]any{"message": "boom"}, nil)
		_, err := c.Balance(context.Background())
		if err == nil {
			t.Fatalf("status %d: expected error", tc.status)
		}

		var e *Error
		if !errors.As(err, &e) {
			t.Fatalf("status %d: error is not *hellio.Error: %v", tc.status, err)
		}
		if e.StatusCode != tc.status {
			t.Errorf("status %d: StatusCode = %d", tc.status, e.StatusCode)
		}
		if e.Kind != tc.kind {
			t.Errorf("status %d: Kind = %d, want %d", tc.status, e.Kind, tc.kind)
		}
		if e.Message != "boom" {
			t.Errorf("status %d: Message = %q", tc.status, e.Message)
		}
		if tc.sentinel != nil && !errors.Is(err, tc.sentinel) {
			t.Errorf("status %d: errors.Is against sentinel failed", tc.status)
		}
	}
}

func TestValidationErrorsAccessor(t *testing.T) {
	c := newTestClient(t, 422, map[string]any{
		"message": "The given data was invalid.",
		"errors":  map[string]any{"recipients": []any{"required"}},
	}, nil)
	_, err := c.SMS(context.Background(), nil, "hi", "S", "")
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *hellio.Error, got %v", err)
	}
	if e.Errors()["recipients"] == nil {
		t.Errorf("expected validation details under recipients, got %v", e.Errors())
	}
}

func toStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(string))
	}
	return out
}
