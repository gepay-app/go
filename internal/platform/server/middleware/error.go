package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"app/internal/platform/apperr"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ErrorHandler renders every failure in one shape:
//
//	{"error":{"code":"...","message":"...","details":...}}
//
// apperr errors bring their status via Kind; Echo's own errors keep their
// status. Anything else becomes a generic 500 (its cause is logged by AccessLog,
// not returned).
func ErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if res, _ := echo.UnwrapResponse(c.Response()); res != nil && res.Committed {
			return
		}
		status, payload := renderError(err)
		_ = c.JSON(status, errorBody{Error: payload})
	}
}

func renderError(err error) (int, errorPayload) {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return statusFor(appErr.Kind), errorPayload{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		}
	}

	status := echo.StatusCode(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	message := http.StatusText(status)
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr.Message != "" {
		message = httpErr.Message
	}
	return status, errorPayload{Code: codeForStatus(status), Message: message}
}

func statusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "error"
	}
}
