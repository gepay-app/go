package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"app/internal/platform/apperr"
)

func newTest(t *testing.T, opts ...Options) (*echo.Echo, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return New(slog.New(slog.NewTextHandler(&buf, nil)), o), &buf
}

func TestHealthIsLoggedWithRequestID(t *testing.T) {
	e, buf := newTest(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	id := rec.Header().Get(echo.HeaderXRequestID)
	if id == "" {
		t.Fatal("response is missing the request id header")
	}
	logs := buf.String()
	if !strings.Contains(logs, "request_id="+id) {
		t.Fatalf("access log missing request id %q:\n%s", id, logs)
	}
	if !strings.Contains(logs, "status=200") {
		t.Fatalf("access log missing status:\n%s", logs)
	}
}

func TestEachRequestGetsItsOwnID(t *testing.T) {
	e, _ := newTest(t)

	seen := make(map[string]bool, 5)
	for range 5 {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		id := rec.Header().Get(echo.HeaderXRequestID)
		if seen[id] {
			t.Fatalf("same request id reused across requests: %q", id)
		}
		seen[id] = true
	}
}

func TestInboundRequestIDIsPropagated(t *testing.T) {
	e, buf := newTest(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(echo.HeaderXRequestID, "given-id")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if got := rec.Header().Get(echo.HeaderXRequestID); got != "given-id" {
		t.Fatalf("response header = %q, want given-id", got)
	}
	if !strings.Contains(buf.String(), "request_id=given-id") {
		t.Fatalf("log did not reuse the inbound id:\n%s", buf.String())
	}
}

func TestPanicReturns500AndLogsOnce(t *testing.T) {
	e, buf := newTest(t)
	e.GET("/boom", func(*echo.Context) error { panic("kaboom") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatalf("panic leaked to the client: %q", rec.Body.String())
	}
	logs := buf.String()
	if n := strings.Count(logs, "level=ERROR"); n != 1 {
		t.Fatalf("want exactly one ERROR line, got %d:\n%s", n, logs)
	}
	if !strings.Contains(logs, "request_id=") || !strings.Contains(logs, "kaboom") {
		t.Fatalf("panic not logged with request id and cause:\n%s", logs)
	}
}

func TestReadyzReportsDependencyState(t *testing.T) {
	e, _ := newTest(t, Options{Dependencies: []Dependency{
		{Name: "postgres", Check: func(context.Context) error { return nil }},
		{Name: "redis", Check: func(context.Context) error { return errors.New("down") }},
	}})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "unavailable" || body.Checks["postgres"] != "ok" || body.Checks["redis"] != "down" {
		t.Fatalf("body = %+v", body)
	}
}

func TestReadyzOK(t *testing.T) {
	e, _ := newTest(t, Options{Dependencies: []Dependency{
		{Name: "postgres", Check: func(context.Context) error { return nil }},
	}})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRejectsNonJSONBody(t *testing.T) {
	e, _ := newTest(t)
	e.POST("/x", func(c *echo.Context) error {
		var body struct {
			A int `json:"a"`
		}
		if err := c.Bind(&body); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("a=1"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415 (%s)", rec.Code, rec.Body.String())
	}
	if code := decodeError(t, rec).Code; code != "unsupported_media_type" {
		t.Fatalf("code = %q", code)
	}
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"details"`
}

type errorBody struct {
	Error errorResponse `json:"error"`
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the error envelope: %v (%q)", err, rec.Body.String())
	}
	return body.Error
}

func TestHTTPErrorBecomesJSON(t *testing.T) {
	e, _ := newTest(t)
	e.GET("/teapot", func(*echo.Context) error { return echo.NewHTTPError(http.StatusTeapot, "nope") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/teapot", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
	if msg := decodeError(t, rec).Message; msg != "nope" {
		t.Fatalf("message = %q, want nope", msg)
	}
}

func TestApplicationErrorMapsStatusAndCode(t *testing.T) {
	e, _ := newTest(t)
	e.GET("/gone", func(*echo.Context) error {
		return apperr.NotFound("user_not_found", "user not found")
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gone", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Code != "user_not_found" || got.Message != "user not found" {
		t.Fatalf("got %+v", got)
	}
	if strings.Contains(rec.Body.String(), "details") {
		t.Fatalf("empty details should be omitted: %q", rec.Body.String())
	}
}

func TestValidationErrorCarriesFieldDetails(t *testing.T) {
	e, _ := newTest(t)
	e.POST("/users", func(*echo.Context) error {
		return apperr.Validation(
			apperr.FieldError{Field: "email", Message: "must be a valid email"},
			apperr.FieldError{Field: "age", Message: "must be at least 18"},
		)
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	got := decodeError(t, rec)
	if got.Code != "validation_failed" {
		t.Fatalf("code = %q, want validation_failed", got.Code)
	}
	if len(got.Details) != 2 || got.Details[0].Field != "email" || got.Details[1].Field != "age" {
		t.Fatalf("details = %+v", got.Details)
	}
}

func TestInternalErrorHidesCauseButLogsIt(t *testing.T) {
	e, buf := newTest(t)
	e.GET("/fail", func(*echo.Context) error {
		return apperr.Wrap(errors.New("db exploded"), apperr.KindInternal, "db_error", "internal error")
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fail", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "db exploded") {
		t.Fatalf("cause leaked to the client: %q", rec.Body.String())
	}
	if !strings.Contains(buf.String(), "cause=\"db exploded\"") {
		t.Fatalf("cause not logged:\n%s", buf.String())
	}
}
