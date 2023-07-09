package pkg

import (
	"golang.org/x/crypto/bcrypt"
)

func CheckPassword(Hash string, Password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(Hash), []byte(Password))
	if err != nil {
		return false
	}
	return true
}
