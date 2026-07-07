# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-07-07

### Added
- USSD support under `client.USSD`, decoding responses into typed structs:
  `Pricing`, `Availability`, `Apps` / `CreateApp` / `UpdateApp` / `DeleteApp`,
  `Extensions` / `RentExtension` / `ReleaseExtension`, `Sessions` / `Session`,
  and `Simulate`. Cursor pagination on the list endpoints returns the next
  cursor. New types: `USSDApp`, `USSDAppInput`, `USSDExtension`, `USSDSession`,
  `USSDPricing`, `USSDSessionPrice`, `USSDExtensionPrice`, `USSDAvailability`,
  `USSDSimulateRequest`, and `USSDSimulateResult`.
- `KindConflict` (409) with the `ErrConflict` sentinel, returned when renting an
  unavailable extension (body error `extension_unavailable`). A rent shortfall
  keeps returning `ErrInsufficientBalance` (402).

### Fixed
- Test harness reset the recorded request body per request so reused recorders no
  longer retain keys from an earlier call.

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
