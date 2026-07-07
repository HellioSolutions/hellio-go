# Hellio Messaging - Official Go SDK

[![tests](https://github.com/HellioSolutions/hellio-go/actions/workflows/test.yml/badge.svg)](https://github.com/HellioSolutions/hellio-go/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/HellioSolutions/hellio-go.svg)](https://pkg.go.dev/github.com/HellioSolutions/hellio-go)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Go client for the [Hellio Messaging](https://helliomessaging.com) API v1:
**SMS**, **OTP** (SMS / email / voice), **Voice broadcasts**, **Number Lookup (HLR)**,
**Email Verification**, **USSD**, and **Webhooks**. Standard library only, no
third-party dependencies.

## Install
```bash
go get github.com/HellioSolutions/hellio-go
```

## Configure
Generate a token in your dashboard: **Settings -> API -> Generate API token**.
Pass it to `NewClient`, or set environment variables and let the client read them:
```dotenv
HELLIO_BASE_URL=https://api.helliomessaging.com/v1
HELLIO_API_TOKEN=your-token-here
HELLIO_DEFAULT_SENDER=HellioSMS
```

```go
import "github.com/HellioSolutions/hellio-go"

// Explicit token plus options.
client := hellio.NewClient("your-token-here",
    hellio.WithDefaultSender("HellioSMS"),
    hellio.WithTimeout(15*time.Second),
)

// Or read HELLIO_API_TOKEN / HELLIO_BASE_URL / HELLIO_DEFAULT_SENDER from the env.
client := hellio.NewClient("")
```

Available options: `WithBaseURL`, `WithTimeout`, `WithDefaultSender`, `WithHTTPClient`.

Every method takes a `context.Context` first and returns `map[string]any` (payloads
live under the `data` key), except `Verify` (a `bool`) and the `client.USSD.*`
methods, which return typed structs. See [USSD](#ussd).

## Usage

```go
ctx := context.Background()

// Account
client.Balance(ctx)          // map[data:map[balance:195.0000 available:194.65 ...]]
client.Pricing(ctx, "GH")    // optional ISO-2 country filter, "" for all

// SMS (recipients: single numbers or comma-separated strings, flattened to a list)
client.SMS(ctx, []string{"233241234567"}, "Hello!", "", "")             // default sender, no gateway
client.SMS(ctx, []string{"233241234567", "233201234567"}, "Hi all", "HellioSMS", "")
client.SMS(ctx, hellio.Recipients("233241234567,233201234567"), "Hi", "HellioSMS", "")
client.Message(ctx, 1024)    // delivery status
client.Campaign(ctx, 1024)   // campaign summary

// OTP - sender (Sender ID) is REQUIRED for sms/voice and must be approved on your
// account. length (4-10 digits) and expiry (minutes) are optional; pass 0 to omit.
client.OTP(ctx, "233241234567", "HellioSMS", "sms", "", 0, 0, "")   // SMS
client.OTP(ctx, "233241234567", "HellioSMS", "voice", "", 0, 0, "") // Voice (TTS reads the code)
client.OTP(ctx, "233241234567", "HellioSMS", "sms", "login", 6, 10, "") // custom length / expiry
client.OTP(ctx, "user@example.com", "", "email", "", 0, 0, "")     // Email (no sender)

client.Verify(ctx, "233241234567", "123456", "sms")                // bool convenience
client.VerifyOTP(ctx, "user@example.com", "123456", "email")       // full response

// Voice broadcast - text (we TTS it) or a hosted audio URL
client.Voice(ctx, []string{"233241234567"}, "HELLIO", "Your code is 1 2 3 4", "", "")
client.Voice(ctx, []string{"233241234567"}, "HELLIO", "", "https://cdn.example.com/promo.mp3", "")
client.VoiceStatus(ctx, 42)

// Number lookup (HLR) - async; poll results
client.Lookup(ctx, []string{"233241234567"})
client.Lookups(ctx)
client.LookupResult(ctx, 5)

// Email verification
client.VerifyEmail(ctx, []string{"user@gmail.com", "bad@nodomain.invalid"})

// Webhooks (receive delivery reports)
client.CreateWebhook(ctx, "https://your-app.com/hooks/hellio", []string{"message.delivered", "message.failed"})
client.Webhooks(ctx)
client.DeleteWebhook(ctx, 1)
```

## USSD

The USSD endpoints are grouped under `client.USSD`. Unlike the other methods,
they decode the response `data` into typed structs (`*hellio.USSDApp`,
`*hellio.USSDExtension`, `*hellio.USSDSession`, `*hellio.USSDPricing`, and so on).
Access requires a token with the `ussd` ability.

A USSD app owns a callback URL. When a subscriber dials your extension, Hellio
POSTs the session event to that URL (`sessionId`, `msisdn`, `serviceCode`,
`input`, `sequence`, `mode`), signed with the header
`X-Hellio-Signature = HMAC-SHA256(rawBody, app.secret)`; your app replies with a
`{ message, action }` body where `action` is `"continue"` or `"end"`.

```go
ctx := context.Background()

// Pricing and availability
price, _ := client.USSD.Pricing(ctx)             // *hellio.USSDPricing
fmt.Println(price.ShortCode, price.SessionPrices)
avail, _ := client.USSD.Availability(ctx, "100") // is code 100 rentable?
if avail.Available {
    fmt.Println("monthly price:", *avail.MonthlyPrice) // nil when unavailable
}

// Apps (List / Create / Update / Delete). List is cursor-paginated: pass "" for
// the first page; nextCursor is "" when there are no more pages.
apps, nextCursor, _ := client.USSD.Apps(ctx, "")
app, _ := client.USSD.CreateApp(ctx, hellio.USSDAppInput{
    Name:        "Bank menu",
    CallbackURL: "https://your-app.com/ussd",
})
fmt.Println(app.ID, app.Secret) // keep Secret to verify X-Hellio-Signature

active := false
client.USSD.UpdateApp(ctx, app.ID, hellio.USSDAppInput{
    Name:        "Bank menu",
    CallbackURL: "https://your-app.com/ussd",
    Active:      &active, // pause inbound delivery
})
client.USSD.DeleteApp(ctx, app.ID)

// Extensions (List / Rent / Release). Pass appID 0 to rent unassigned.
exts, _, _ := client.USSD.Extensions(ctx, "")
ext, err := client.USSD.RentExtension(ctx, "100", app.ID)
if err != nil {
    switch {
    case errors.Is(err, hellio.ErrConflict):           // 409 extension_unavailable
    case errors.Is(err, hellio.ErrInsufficientBalance): // 402 insufficient_balance
    }
}
client.USSD.ReleaseExtension(ctx, ext.ID)

// Sessions (List / Get). Filter by status ("" for all) and paginate with cursor.
sessions, _, _ := client.USSD.Sessions(ctx, "ended", "")
session, _ := client.USSD.Session(ctx, sessions[0].ID)

// Simulate a dialog against your callback without a real subscriber.
res, _ := client.USSD.Simulate(ctx, hellio.USSDSimulateRequest{
    Msisdn:      "233241234567",
    ServiceCode: "*713*100#",
    NewSession:  true,
})
fmt.Println(res.Message, res.Action) // action: "continue" or "end"
```

### Reading a response
Responses are plain maps. Reach into `data` with a type assertion:
```go
res, err := client.Balance(ctx)
if err != nil {
    log.Fatal(err)
}
data := res["data"].(map[string]any)
fmt.Println(data["balance"])
```

### Recipient normalization
`SMS`, `Voice`, `Lookup`, and `VerifyEmail` accept a `[]string`. Each entry may be a
single value or a comma-separated string; entries are trimmed and flattened. The
`hellio.Recipients(...)` helper does the same and is handy for building lists from
mixed input.

## Error handling
Non-2xx responses return a typed `*hellio.Error` carrying `StatusCode`, `Message`,
`Body`, and a `Kind`. Use `errors.As` to read the fields or `errors.Is` against a
sentinel:

| Sentinel | Kind | Status |
|---|---|---|
| `hellio.ErrInvalidApiToken` | `KindInvalidApiToken` | 401 |
| `hellio.ErrInsufficientBalance` | `KindInsufficientBalance` | 402 |
| `hellio.ErrConflict` | `KindConflict` | 409 |
| `hellio.ErrValidation` | `KindValidation` | 422 |
| `hellio.ErrRateLimit` | `KindRateLimit` | 429 |
| `hellio.ErrServiceUnavailable` | `KindServiceUnavailable` | 503 |
| (base error) | `KindGeneric` | other |

```go
res, err := client.SMS(ctx, []string{"233241234567"}, "Hi", "HellioSMS", "")
if err != nil {
    switch {
    case errors.Is(err, hellio.ErrInsufficientBalance):
        // top up
    case errors.Is(err, hellio.ErrRateLimit):
        // back off and retry
    default:
        var e *hellio.Error
        if errors.As(err, &e) {
            fmt.Println(e.StatusCode, e.Message, e.Errors()) // Errors() holds 422 details
        }
    }
}
```

`Verify` is the exception: a 422 (wrong code) is reported as `(false, nil)` rather
than an error.

Rate limit: **120 requests/minute** per token.

## Testing
```bash
go vet ./...
go test ./...
```
Tests use `net/http/httptest` to mock the API; inject a client with
`hellio.WithHTTPClient(...)` to point at your own server.

## License
MIT. See [LICENSE](LICENSE).
