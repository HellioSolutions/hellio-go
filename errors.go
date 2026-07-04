package hellio

import "fmt"

// Kind classifies a Hellio API error by its HTTP status. Callers can switch on it
// or use errors.Is against the exported sentinels below.
type Kind int

const (
	// KindGeneric is any non-2xx response that is not one of the specific kinds.
	KindGeneric Kind = iota
	// KindInvalidApiToken maps to HTTP 401.
	KindInvalidApiToken
	// KindInsufficientBalance maps to HTTP 402.
	KindInsufficientBalance
	// KindValidation maps to HTTP 422.
	KindValidation
	// KindRateLimit maps to HTTP 429.
	KindRateLimit
	// KindServiceUnavailable maps to HTTP 503 (a service switched off by an admin,
	// or the API paused). Transient: retry later.
	KindServiceUnavailable
)

// Error is returned for every non-2xx response. It carries the HTTP status code,
// a human message (from the body "message" field or a default), the parsed body,
// and a Kind for convenient switching. Use errors.As to reach these fields:
//
//	var e *hellio.Error
//	if errors.As(err, &e) { fmt.Println(e.StatusCode, e.Message) }
//
// Or errors.Is against a sentinel:
//
//	if errors.Is(err, hellio.ErrRateLimit) { time.Sleep(time.Second) }
type Error struct {
	Kind       Kind
	StatusCode int
	Message    string
	// Body is the decoded JSON response, if any.
	Body map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("hellio: request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("hellio: %s (status %d)", e.Message, e.StatusCode)
}

// Is lets errors.Is match by Kind, so callers can compare against the sentinels.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Kind == t.Kind
}

// Errors returns the validation details a 422 response carries under "errors",
// or nil when the body has no such field.
func (e *Error) Errors() map[string]any {
	if e.Body == nil {
		return nil
	}
	if v, ok := e.Body["errors"].(map[string]any); ok {
		return v
	}
	return nil
}

// Sentinel errors for use with errors.Is. Each matches any Error of the same Kind.
var (
	ErrInvalidApiToken     = &Error{Kind: KindInvalidApiToken}
	ErrInsufficientBalance = &Error{Kind: KindInsufficientBalance}
	ErrValidation          = &Error{Kind: KindValidation}
	ErrRateLimit           = &Error{Kind: KindRateLimit}
	ErrServiceUnavailable  = &Error{Kind: KindServiceUnavailable}
)

func newError(status int, message string, body map[string]any) *Error {
	kind := KindGeneric
	switch status {
	case 401:
		kind = KindInvalidApiToken
	case 402:
		kind = KindInsufficientBalance
	case 422:
		kind = KindValidation
	case 429:
		kind = KindRateLimit
	case 503:
		kind = KindServiceUnavailable
	}
	return &Error{Kind: kind, StatusCode: status, Message: message, Body: body}
}
