package config

import (
	"github.com/denisbrodbeck/machineid"
	"gorm.io/gorm"
)

const (
	UpdatePath = "./updates/"
	JwtSecret  = "secret"
	AppKey     = "hospital"
)

var (
	DB             *gorm.DB
	HashedID, err  = machineid.ProtectedID(AppKey)
	ActivationCode = ""
	CurrencyCode   = "N"
	LoggedInID     = map[string]string{"dd": "dd"}
	// LoggedInRole = 0
)

type LoggedInStruct struct {
	ID       string `json:"id"`
	Role     uint   `json:"role"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

var LoggedIn = LoggedInStruct{
	Role:     0,
	Username: "",
	Name:     "",
	ID:       "",
}

// initial db settings
var (
	InitSettings = map[string]string{
		"installation_code": HashedID,
		"activation_code":   "",
		"name":              "",
		"address":           "",
		"phone":             "",
		"email":             "",
		"logo":              "",
		"currency_code":     "",
	}
)

const (
	ADMIN_ROLE     = 1
	SALES_REP_ROLE = 2
)

type BodyStructure map[string]string
