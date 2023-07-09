package helpers

import (
	"crypto/md5"
	"encoding/hex"
)

func GetHash(input string) string {
	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}
