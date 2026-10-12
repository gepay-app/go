package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v5"
)

// Recover turns a panic into a 500 error carrying its stack trace. It does not
// log: AccessLog (placed outside it) records the failure with the request id,
// and Echo's default error handler keeps the stack away from the client.
func Recover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if r == http.ErrAbortHandler {
						panic(r)
					}
					err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
				}
			}()
			return next(c)
		}
	}
}
