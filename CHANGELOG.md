# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-05

### Added
- Initial release of the official Hellio Messaging Go SDK.
- `Client` with functional options: `WithBaseURL`, `WithTimeout`,
  `WithDefaultSender`, `WithHTTPClient`. Env fallbacks for `HELLIO_API_TOKEN`,
  `HELLIO_BASE_URL`, and `HELLIO_DEFAULT_SENDER`.
- Methods: `Balance`, `Pricing`, `SMS`, `Message`, `Campaign`, `OTP`, `VerifyOTP`,
  `Verify`, `Voice`, `VoiceStatus`, `Lookup`, `Lookups`, `LookupResult`,
  `VerifyEmail`, `Webhooks`, `CreateWebhook`, `DeleteWebhook`.
- Recipient normalization via `Recipients(...)`.
- Typed `*Error` with `StatusCode`, `Message`, `Body`, and `Kind`, plus sentinels
  `ErrInvalidApiToken` (401), `ErrInsufficientBalance` (402), `ErrValidation` (422),
  and `ErrRateLimit` (429) for use with `errors.Is` and `errors.As`.
- Tests using `net/http/httptest`.
- GitHub Actions workflow running `go vet` and `go test` on Go 1.21 and 1.22.
