package controllers

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
	"github.com/onumahkalusamuel/zebiventor/pkg"
)

// Internal struct
type General struct {
	ctx context.Context
}

func GeneralApp() *General {
	return &General{}
}

// Installation Code
func (g *General) InstallationCode() string {
	return config.HashedID
}

// Check activation
func (g *General) CheckActivation() uint {
	activationCode := config.ActivationCode
	if activationCode == "" {
		settings := &models.Settings{Setting: "activation_code"}
		settings.Read()
		activationCode = settings.Value
		config.ActivationCode = activationCode
	}

	checkActivationCode := pkg.GenActivationCode(config.HashedID)

	if activationCode != checkActivationCode {
		return 1
	}

	// check if admin has been created
	staff := models.Staff{Role: config.ADMIN_ROLE}
	staff.Read()
	if staff.ID == "" {
		return 2
	}

	// check if details have been saved.

	settings := &models.Settings{Setting: "name"}
	settings.Read()
	hospitalName := settings.Value

	if hospitalName == "" {
		return 3
	}

	// if all is good, continue
	return 4
}

// Login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (g *General) Login(loginRequest *LoginRequest) bool {

	staff := models.Staff{
		Username: loginRequest.Username,
	}

	staff.Read()

	if staff.ID == "" || !pkg.CheckPassword(staff.Password, loginRequest.Password) {
		return false
	}

	// Set values claims
	config.LoggedIn.Username = staff.Username
	config.LoggedIn.ID = staff.ID
	config.LoggedIn.Name = staff.Name
	config.LoggedIn.Role = staff.Role

	return true
}

// Setup
type SetupRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	// Logo    string `json:"logo"`
}

func (g *General) Setup(setupRequest *SetupRequest) map[string]interface{} {

	check := &models.Settings{Setting: "name"}
	check.Read()

	if check.Value != "" {
		return echo.Map{
			"code":    1,
			"success": false,
			"message": " record already setup.",
		}
	}

	if setupRequest.Name == "" || setupRequest.Address == "" {
		return echo.Map{
			"code":    2,
			"success": false,
			"message": "Please provide valid details - 2",
		}
	}

	var setting *models.Settings
	// name
	setting = &models.Settings{Setting: "name"}
	setting.Read()
	setting.UpdateSingle("Value", setupRequest.Name)

	// address
	setting = &models.Settings{Setting: "address"}
	setting.Read()
	setting.UpdateSingle("Value", setupRequest.Address)

	// email
	setting = &models.Settings{Setting: "email"}
	setting.Read()
	setting.UpdateSingle("Value", setupRequest.Email)

	// phone
	setting = &models.Settings{Setting: "phone"}
	setting.Read()
	setting.UpdateSingle("Value", setupRequest.Phone)

	// image
	// setting = &models.Settings{Setting: "logo"}
	// setting.Read()
	// setting.UpdateSingle("Value", setupRequest.Logo)

	// return
	return echo.Map{
		"code":    3,
		"success": true,
		"message": " record updated successfully.",
	}
}

// Details

func (g *General) StoreDetails() map[string]interface{} {

	store := echo.Map{
		"name":    "",
		"address": "",
		"email":   "",
		"phone":   "",
		"logo":    "",
	}
	var setting *models.Settings

	// name
	setting = &models.Settings{Setting: "name"}
	setting.Read()
	store["name"] = setting.Value

	// address
	setting = &models.Settings{Setting: "address"}
	setting.Read()
	store["address"] = setting.Value

	// email
	setting = &models.Settings{Setting: "email"}
	setting.Read()
	store["email"] = setting.Value

	// phone
	setting = &models.Settings{Setting: "phone"}
	setting.Read()
	store["phone"] = setting.Value

	// logo
	setting = &models.Settings{Setting: "logo"}
	setting.Read()
	store["logo"] = "http//localhost:8889/" + setting.Value

	// return
	return store
}

// Activation code request
type ActivationRequest struct {
	InstallationCode string `json:"installation_code"`
}

func (g *General) GetActivationCode(activationRequest *ActivationRequest) map[string]interface{} {
	if len(activationRequest.InstallationCode) < 10 {
		return echo.Map{
			"code":    3,
			"success": false,
			"message": "Invalid installation code",
		}
	}

	activationCode := pkg.GenActivationCode(activationRequest.InstallationCode)

	return echo.Map{
		"success":         true,
		"activation_code": activationCode,
	}
}

// create admin
type CreateAdminRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (g *General) CreateAdmin(createAdminRequest *CreateAdminRequest) map[string]interface{} {
	checkadmin := &models.Staff{Role: config.ADMIN_ROLE}
	checkadmin.Read()

	if checkadmin.ID != "" {
		return echo.Map{
			"code":    1,
			"success": false,
			"message": "Admin user already exist.",
		}
	}

	if createAdminRequest.Name == "" || createAdminRequest.Password == "" {
		return echo.Map{
			"code":    2,
			"success": false,
			"message": "Please provide valid details",
		}
	}

	//continue
	admin := &models.Staff{
		Name:     createAdminRequest.Name,
		Username: createAdminRequest.Username,
		Password: pkg.HashPassword(createAdminRequest.Password),
		Role:     config.ADMIN_ROLE,
	}

	if err := admin.Create(); err != nil {
		return echo.Map{
			"code":    3,
			"success": false,
			"message": "Error occured: " + err.Error(),
		}
	}

	return echo.Map{"success": true, "message": "Admin created successfully."}
}

// activate
type ActivateRequest struct {
	ActivationCode string `json:"activation_code"`
}

func (g *General) Activate(activateRequest ActivateRequest) map[string]interface{} {
	installationCode := config.HashedID
	activationCode := activateRequest.ActivationCode
	checkActivationCode := pkg.GenActivationCode(installationCode)

	if len(installationCode) < 10 || len(activationCode) != 14 || activationCode != checkActivationCode {
		return echo.Map{
			"success": false,
			"message": "Invalid activation code",
		}
	}

	//update settings
	settings := &models.Settings{Setting: "activation_code"}
	settings.Read()
	settings.UpdateSingle("value", activationCode)

	// set activation in config
	config.ActivationCode = activationCode

	return echo.Map{
		"success": true,
		"message": "System activated successfully.",
	}
}

// Check login
func (g *General) CheckLogin() config.LoggedInStruct {
	return config.LoggedIn
}
