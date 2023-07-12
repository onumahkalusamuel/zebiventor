package helpers

import (
	"encoding/base64"
	"fmt"
	"os"
)

func SaveImage(logoString string, logoType string) (string, error) {

	dec, err := base64.StdEncoding.DecodeString(logoString)
	if err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("store_logo.%s", logoType)
	fmt.Println(fileName)

	f, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(dec); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}

	fmt.Println(fileName)
	return fileName, nil
}
