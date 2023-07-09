package middleware

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
)

func AppendJwtClaims() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := c.Get("user").(*jwt.Token)
			claims := user.Claims.(jwt.MapClaims)
			c.Set("ID", claims["id"].(string))
			c.Set("Username", claims["username"].(string))
			c.Set("Lastname", claims["lastname"].(string))
			c.Set("Role", claims["role"].(float64))
			return next(c)
		}
	}
}
