package captcha

import (
	"errors"
	"fmt"
)

// APIError represents a {errorId: 1, errorCode, errorDescription} response from the 2captcha
// JSON API v2. errorCode is a stable enum (e.g. "ERROR_ZERO_BALANCE"); HTTP status stays 200
// even on failure, so callers must branch on this type rather than the transport status code.
type APIError struct {
	Code        string
	Description string
}

func (e *APIError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("2captcha: %s: %s", e.Code, e.Description)
	}
	return fmt.Sprintf("2captcha: %s", e.Code)
}

// fatalCodes cannot be fixed by retrying: bad credentials, no funds, or a malformed/unsupported
// request. Retrying only burns time and, for image tasks, may burn balance again.
var fatalCodes = map[string]bool{
	"ERROR_KEY_DOES_NOT_EXIST":        true,
	"ERROR_KEY_IS_EMPTY":              true,
	"ERROR_ZERO_BALANCE":              true,
	"ERROR_IP_BLOCKED":                true,
	"ERROR_ACCOUNT_SUSPENDED":         true,
	"ERROR_NO_SUCH_METHOD":            true,
	"ERROR_TASK_ABSENT":               true,
	"ERROR_TASK_NOT_SUPPORTED":        true,
	"ERROR_BAD_PARAMETERS":            true,
	"ERROR_PAGEURL":                   true,
	"ERROR_RECAPTCHA_INVALID_SITEKEY": true,
	"ERROR_ZERO_CAPTCHA_FILESIZE":     true,
	"ERROR_TOO_BIG_CAPTCHA_FILESIZE":  true,
	"ERROR_IMAGE_TYPE_NOT_SUPPORTED":  true,
	"ERROR_BAD_IMGINSTRUCTIONS":       true,
	"ERROR_WRONG_USER_KEY":            true,
}

// IsFatal reports whether err represents a 2captcha condition that will not resolve by retrying
// (e.g. bad API key, zero balance). Any other error — including transport failures, timeouts,
// and ERROR_CAPTCHA_UNSOLVABLE — is treated as retryable.
func IsFatal(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return fatalCodes[apiErr.Code]
}
