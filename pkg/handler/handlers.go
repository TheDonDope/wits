package handler

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// render provides a shorthand function to render the template of a Templ component.
func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response())
}

// hxRedirect provides a shorthand function to redirect the user with HX-Redirect header.
func hxRedirect(c echo.Context, to string) error {
	slog.Info("💬 🤝 (pkg/handler/handlers.go) 🔄 hxRedirect()", "to", to)
	if len(c.Request().Header.Get("HX-Request")) > 0 {
		c.Response().Header().Set("HX-Redirect", to)
		return nil
	}
	return c.Redirect(http.StatusSeeOther, to)
}
