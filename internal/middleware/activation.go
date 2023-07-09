package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
	"github.com/onumahkalusamuel/zebiventor/pkg"
)

func ActivationAndInitialSetup() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			path := c.Request().URL.Path

			// check if hospital details have been saved.
			if path == "/api/get-activation-code" {
				return next(c)
			}

			// check activation
			if path == "/api/activate" {
				return next(c)
			}

			activationCode := config.ActivationCode
			if activationCode == "" {
				settings := &models.Settings{Setting: "activation_code"}
				settings.Read()
				activationCode = settings.Value
				config.ActivationCode = activationCode
			}

			checkActivationCode := pkg.GenActivationCode(config.HashedID)

			if activationCode != checkActivationCode {
				return c.JSON(http.StatusUnauthorized, echo.Map{
					"code":              1,
					"message":           "Software not activated yet.",
					"installation_code": config.HashedID,
				})
			}

			// check if admin has been created
			if path == "/api/create-admin" {
				return next(c)
			}

			staff := models.Staff{Role: config.ADMIN_ROLE}
			staff.Read()
			if staff.ID == "" {
				return c.JSON(http.StatusUnauthorized, echo.Map{
					"code":    2,
					"message": "Admin user not created.",
				})
			}

			// check if hospital details have been saved.
			if path == "/api/hospital-setup" ||
				path == "/api/get-activation-code" {
				return next(c)
			}

			settings := &models.Settings{Setting: "hospital_name"}
			settings.Read()
			hospitalName := settings.Value

			if hospitalName == "" {
				return c.JSON(http.StatusUnauthorized, echo.Map{
					"code":    3,
					"message": "Hospital details not saved yet.",
				})
			}

			// if all is good, continue
			return next(c)
		}
	}
}
