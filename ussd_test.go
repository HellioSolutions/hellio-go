package hellio

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestUSSDPricing(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"short_code": "713",
		"currency":   "GHS",
		"session_prices": []any{
			map[string]any{"network": "MTN", "slug": "mtn", "session_price": "0.0300"},
		},
		"extension_prices": []any{
			map[string]any{"length": 3, "monthly_price": "50.00"},
		},
	}}, &rec)

	p, err := c.USSD.Pricing(context.Background())
	if err != nil {
		t.Fatalf("Pricing error: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/ussd/pricing" {
		t.Errorf("request = %s %s, want GET /ussd/pricing", rec.method, rec.path)
	}
	if p.ShortCode != "713" || p.Currency != "GHS" {
		t.Errorf("pricing header = %+v", p)
	}
	if len(p.SessionPrices) != 1 || p.SessionPrices[0].Network != "MTN" || p.SessionPrices[0].SessionPrice != "0.0300" {
		t.Errorf("session prices = %+v", p.SessionPrices)
	}
	if len(p.ExtensionPrices) != 1 || p.ExtensionPrices[0].Length != 3 || p.ExtensionPrices[0].MonthlyPrice != "50.00" {
		t.Errorf("extension prices = %+v", p.ExtensionPrices)
	}
}

func TestUSSDAvailability(t *testing.T) {
	var rec recorded
	price := "50.00"
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"code": "100", "valid": true, "available": true, "monthly_price": price,
	}}, &rec)

	a, err := c.USSD.Availability(context.Background(), "100")
	if err != nil {
		t.Fatalf("Availability error: %v", err)
	}
	if rec.path != "/ussd/pricing/availability" || rec.query != "code=100" {
		t.Errorf("request = %s?%s", rec.path, rec.query)
	}
	if !a.Valid || !a.Available || a.MonthlyPrice == nil || *a.MonthlyPrice != price {
		t.Errorf("availability = %+v", a)
	}
}

func TestUSSDAvailabilityNullPrice(t *testing.T) {
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"code": "100", "valid": true, "available": false, "monthly_price": nil,
	}}, nil)

	a, err := c.USSD.Availability(context.Background(), "100")
	if err != nil {
		t.Fatalf("Availability error: %v", err)
	}
	if a.MonthlyPrice != nil {
		t.Errorf("monthly_price = %v, want nil", *a.MonthlyPrice)
	}
}

func TestUSSDApps(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{
		"data": []any{
			map[string]any{"id": "11111111-1111-1111-1111-111111111111", "name": "Bank", "callback_url": "https://x/cb", "mode": "test", "test_secret": "ussk_test_abc", "live_secret": "ussk_live_xyz", "is_live": false, "active": true, "created_at": "2026-07-01"},
		},
		"meta": map[string]any{"next_cursor": "abc"},
	}, &rec)

	apps, next, err := c.USSD.Apps(context.Background(), "")
	if err != nil {
		t.Fatalf("Apps error: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/ussd/apps" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.query != "" {
		t.Errorf("query = %q, want empty on first page", rec.query)
	}
	if next != "abc" {
		t.Errorf("nextCursor = %q, want abc", next)
	}
	if len(apps) != 1 || apps[0].ID != "11111111-1111-1111-1111-111111111111" || apps[0].Name != "Bank" || !apps[0].Active {
		t.Errorf("apps = %+v", apps)
	}
	if apps[0].Mode != "test" || apps[0].TestSecret != "ussk_test_abc" || apps[0].LiveSecret != "ussk_live_xyz" || apps[0].IsLive {
		t.Errorf("app mode/secrets = %+v", apps[0])
	}
}

func TestUSSDAppsCursor(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": []any{}, "meta": map[string]any{"next_cursor": ""}}, &rec)

	if _, _, err := c.USSD.Apps(context.Background(), "abc"); err != nil {
		t.Fatalf("Apps error: %v", err)
	}
	if rec.query != "cursor=abc" {
		t.Errorf("query = %q, want cursor=abc", rec.query)
	}
}

func TestUSSDCreateApp(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 201, map[string]any{"data": map[string]any{
		"id": "7a7b", "name": "Bank", "callback_url": "https://x/cb", "mode": "test", "test_secret": "ussk_test_shh", "live_secret": "ussk_live_shh", "is_live": false, "active": true,
	}}, &rec)

	active := false // must be ignored on create
	app, err := c.USSD.CreateApp(context.Background(), USSDAppInput{Name: "Bank", CallbackURL: "https://x/cb", Active: &active})
	if err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/ussd/apps" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body["name"] != "Bank" || rec.body["callback_url"] != "https://x/cb" {
		t.Errorf("body = %v", rec.body)
	}
	if _, ok := rec.body["active"]; ok {
		t.Errorf("active should be omitted on create, got %v", rec.body["active"])
	}
	if app.ID != "7a7b" || app.Mode != "test" || app.TestSecret != "ussk_test_shh" || app.IsLive {
		t.Errorf("app = %+v", app)
	}
}

func TestUSSDUpdateApp(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"id": "7a7b", "name": "Bank2", "active": false}}, &rec)

	active := false
	app, err := c.USSD.UpdateApp(context.Background(), "7a7b", USSDAppInput{Name: "Bank2", CallbackURL: "https://x/cb", Active: &active})
	if err != nil {
		t.Fatalf("UpdateApp error: %v", err)
	}
	if rec.method != http.MethodPut || rec.path != "/ussd/apps/7a7b" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body["active"] != false {
		t.Errorf("active = %v, want false", rec.body["active"])
	}
	if app.Name != "Bank2" || app.Active {
		t.Errorf("app = %+v", app)
	}
}

func TestUSSDDeleteApp(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 204, nil, &rec)

	if err := c.USSD.DeleteApp(context.Background(), "7a7b"); err != nil {
		t.Fatalf("DeleteApp error: %v", err)
	}
	if rec.method != http.MethodDelete || rec.path != "/ussd/apps/7a7b" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestUSSDSetMode(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"id": "7a7b", "name": "Bank", "mode": "live", "is_live": true, "active": true,
	}}, &rec)

	app, err := c.USSD.SetMode(context.Background(), "7a7b", "live")
	if err != nil {
		t.Fatalf("SetMode error: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/ussd/apps/7a7b/mode" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body["mode"] != "live" {
		t.Errorf("body = %v", rec.body)
	}
	if app.Mode != "live" || !app.IsLive {
		t.Errorf("app = %+v", app)
	}
}

func TestUSSDSetModeExtensionRequired(t *testing.T) {
	c := newTestClient(t, 402, map[string]any{"error": "extension_required"}, nil)

	_, err := c.USSD.SetMode(context.Background(), "7a7b", "live")
	if err == nil {
		t.Fatal("expected error on 402 extension_required")
	}
	if !errors.Is(err, ErrExtensionRequired) {
		t.Errorf("errors.Is(ErrExtensionRequired) = false for %v", err)
	}
	if errors.Is(err, ErrInsufficientBalance) {
		t.Errorf("extension_required must not match ErrInsufficientBalance")
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("not a *hellio.Error: %v", err)
	}
	if e.Kind != KindExtensionRequired || e.StatusCode != 402 {
		t.Errorf("kind/status = %d/%d", e.Kind, e.StatusCode)
	}
	if e.Message != "extension_required" {
		t.Errorf("message = %q, want extension_required", e.Message)
	}
}

func TestUSSDRotateSecret(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"id": "7a7b", "mode": "test", "test_secret": "ussk_test_new", "live_secret": "ussk_live_old",
	}}, &rec)

	app, err := c.USSD.RotateSecret(context.Background(), "7a7b", "test")
	if err != nil {
		t.Fatalf("RotateSecret error: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/ussd/apps/7a7b/rotate-secret" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body["mode"] != "test" {
		t.Errorf("body = %v", rec.body)
	}
	if app.TestSecret != "ussk_test_new" {
		t.Errorf("app = %+v", app)
	}
}

func TestUSSDExtensions(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{
		"data": []any{
			map[string]any{"id": "e3", "code": "100", "dial_string": "*713*100#", "length": 3, "status": "active", "monthly_price": "50.00", "auto_renew": true, "app_id": "7a7b", "expires_at": "2026-08-01"},
		},
	}, &rec)

	exts, next, err := c.USSD.Extensions(context.Background(), "")
	if err != nil {
		t.Fatalf("Extensions error: %v", err)
	}
	if rec.path != "/ussd/extensions" {
		t.Errorf("path = %s", rec.path)
	}
	if next != "" {
		t.Errorf("nextCursor = %q, want empty", next)
	}
	if len(exts) != 1 || exts[0].Code != "100" || exts[0].AppID == nil || *exts[0].AppID != "7a7b" {
		t.Errorf("exts = %+v", exts)
	}
}

func TestUSSDRentExtension(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 201, map[string]any{"data": map[string]any{"id": "e9", "code": "100", "status": "active"}}, &rec)

	ext, err := c.USSD.RentExtension(context.Background(), "100", "7a7b")
	if err != nil {
		t.Fatalf("RentExtension error: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/ussd/extensions" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body["code"] != "100" || rec.body["app_id"] != "7a7b" {
		t.Errorf("body = %v", rec.body)
	}
	if ext.ID != "e9" {
		t.Errorf("ext = %+v", ext)
	}
}

func TestUSSDRentExtensionNoApp(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 201, map[string]any{"data": map[string]any{"id": "e9"}}, &rec)

	if _, err := c.USSD.RentExtension(context.Background(), "100", ""); err != nil {
		t.Fatalf("RentExtension error: %v", err)
	}
	if _, ok := rec.body["app_id"]; ok {
		t.Errorf("app_id should be omitted when empty, got %v", rec.body["app_id"])
	}
}

func TestUSSDRentExtensionConflict(t *testing.T) {
	c := newTestClient(t, 409, map[string]any{"error": "extension_unavailable"}, nil)

	_, err := c.USSD.RentExtension(context.Background(), "100", "")
	if err == nil {
		t.Fatal("expected error on 409")
	}
	if !errors.Is(err, ErrConflict) {
		t.Errorf("errors.Is(ErrConflict) = false for %v", err)
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("not a *hellio.Error: %v", err)
	}
	if e.Kind != KindConflict || e.StatusCode != 409 {
		t.Errorf("kind/status = %d/%d", e.Kind, e.StatusCode)
	}
	if e.Message != "extension_unavailable" || e.Body["error"] != "extension_unavailable" {
		t.Errorf("message/body = %q / %v", e.Message, e.Body)
	}
}

func TestUSSDRentExtensionInsufficientBalance(t *testing.T) {
	c := newTestClient(t, 402, map[string]any{"error": "insufficient_ussd_balance"}, nil)

	_, err := c.USSD.RentExtension(context.Background(), "100", "")
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Errorf("errors.Is(ErrInsufficientBalance) = false for %v", err)
	}
	if errors.Is(err, ErrExtensionRequired) {
		t.Errorf("insufficient_ussd_balance must not match ErrExtensionRequired")
	}
	var e *Error
	if errors.As(err, &e) && e.Message != "insufficient_ussd_balance" {
		t.Errorf("message = %q, want insufficient_ussd_balance", e.Message)
	}
}

func TestUSSDReleaseExtension(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 204, nil, &rec)

	if err := c.USSD.ReleaseExtension(context.Background(), "e9"); err != nil {
		t.Fatalf("ReleaseExtension error: %v", err)
	}
	if rec.method != http.MethodDelete || rec.path != "/ussd/extensions/e9" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestUSSDSessions(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{
		"data": []any{
			map[string]any{"id": "s5", "session_ref": "ref", "msisdn": "233241234567", "service_code": "*713*100#", "status": "ended", "steps": 3, "charge": "0.09", "sandbox": false, "started_at": "a", "ended_at": "b"},
		},
		"meta": map[string]any{"next_cursor": "n2"},
	}, &rec)

	sessions, next, err := c.USSD.Sessions(context.Background(), "ended", "n1")
	if err != nil {
		t.Fatalf("Sessions error: %v", err)
	}
	if rec.path != "/ussd/sessions" {
		t.Errorf("path = %s", rec.path)
	}
	if q := rec.query; q != "cursor=n1&status=ended" {
		t.Errorf("query = %q, want cursor=n1&status=ended", q)
	}
	if next != "n2" {
		t.Errorf("nextCursor = %q, want n2", next)
	}
	if len(sessions) != 1 || sessions[0].ID != "s5" || sessions[0].Steps != 3 || sessions[0].Status != "ended" {
		t.Errorf("sessions = %+v", sessions)
	}
}

func TestUSSDSessionsNoFilters(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": []any{}}, &rec)

	if _, _, err := c.USSD.Sessions(context.Background(), "", ""); err != nil {
		t.Fatalf("Sessions error: %v", err)
	}
	if rec.query != "" {
		t.Errorf("query = %q, want empty", rec.query)
	}
}

func TestUSSDSession(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{"id": "s5", "status": "active"}}, &rec)

	sess, err := c.USSD.Session(context.Background(), "s5")
	if err != nil {
		t.Fatalf("Session error: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/ussd/sessions/s5" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if sess.ID != "s5" || sess.Status != "active" {
		t.Errorf("session = %+v", sess)
	}
}

func TestUSSDSimulate(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"message": "Welcome", "action": "continue", "continue": true,
	}}, &rec)

	code := "*713*100#"
	res, err := c.USSD.Simulate(context.Background(), USSDSimulateRequest{
		AppID: "7a7b", Msisdn: "233241234567", ServiceCode: &code, Input: "", NewSession: true,
	})
	if err != nil {
		t.Fatalf("Simulate error: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/ussd/simulate" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body["app_id"] != "7a7b" || rec.body["new_session"] != true || rec.body["service_code"] != "*713*100#" {
		t.Errorf("body = %v", rec.body)
	}
	if _, ok := rec.body["session_id"]; ok {
		t.Errorf("session_id should be omitted when empty, got %v", rec.body["session_id"])
	}
	if res.Message != "Welcome" || res.Action != "continue" || !res.Continue {
		t.Errorf("result = %+v", res)
	}
}

func TestUSSDSimulateDefaultServiceCode(t *testing.T) {
	var rec recorded
	c := newTestClient(t, 200, map[string]any{"data": map[string]any{
		"message": "Welcome", "action": "continue", "continue": true,
	}}, &rec)

	// ServiceCode left nil: the server defaults it to the shared short code, so it
	// must be omitted from the request body.
	if _, err := c.USSD.Simulate(context.Background(), USSDSimulateRequest{
		AppID: "7a7b", Msisdn: "233241234567", NewSession: true,
	}); err != nil {
		t.Fatalf("Simulate error: %v", err)
	}
	if _, ok := rec.body["service_code"]; ok {
		t.Errorf("service_code should be omitted when nil, got %v", rec.body["service_code"])
	}
}

func TestUSSDSimulateUnknownApp(t *testing.T) {
	c := newTestClient(t, 422, map[string]any{"error": "unknown_app"}, nil)

	_, err := c.USSD.Simulate(context.Background(), USSDSimulateRequest{
		AppID: "nope", Msisdn: "233241234567", NewSession: true,
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("errors.Is(ErrValidation) = false for %v", err)
	}
	var e *Error
	if errors.As(err, &e) && e.Message != "unknown_app" {
		t.Errorf("message = %q, want unknown_app", e.Message)
	}
}
