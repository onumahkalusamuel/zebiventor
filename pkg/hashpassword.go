package pkg

import "golang.org/x/crypto/bcrypt"

func HashPassword(Password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(Password), 4)
	return string(hash)
}
