package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

// jsonBinder accepts only application/json request bodies; any other media type
// is rejected with 415. Requests without a body (GET, DELETE, ...) pass through.
type jsonBinder struct{}

func (jsonBinder) Bind(c *echo.Context, target any) error {
	if req := c.Request(); req.ContentLength != 0 {
		mediaType, _, _ := strings.Cut(req.Header.Get(echo.HeaderContentType), ";")
		if strings.TrimSpace(mediaType) != echo.MIMEApplicationJSON {
			return echo.NewHTTPError(http.StatusUnsupportedMediaType, "only application/json is accepted")
		}
	}
	return (&echo.DefaultBinder{}).Bind(c, target)
}
