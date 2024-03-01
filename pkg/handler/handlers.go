package handler

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// render provides a shorthand function to render the template of a Templ component.
//
// Parameters:
// - c echo.Context: The echo context.
// - component templ.Component: The component to render.
//
// Returns:
// - error: The error if any.
func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response())
}
