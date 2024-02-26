package handler

import (
	"fmt"
	"net/http"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/dashboard"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// DashboardHandler handles the dashboard page.
type DashboardHandler struct{}

// HandleGetDashboard responds to GET on the /dashboard route by rendering the Dashboard component.
func (h *DashboardHandler) HandleGetDashboard(c echo.Context) error {
	accessTokenCookie, err := c.Cookie(auth.AccessTokenCookieName)
	if err != nil {
		fmt.Println("No access token cookie found")
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(accessTokenCookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(auth.GetJWTSecret()), nil
	})
	if err != nil {
		fmt.Println("Error parsing token:", err)
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	fmt.Printf("Token: %v\n", token)
	email := claims["email"].(string)
	return render(c, dashboard.Dashboard(&types.User{Email: email}))
}
