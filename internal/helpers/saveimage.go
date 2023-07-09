package helpers

import (
	"io"
	"os"
	"path"

	"github.com/labstack/echo/v4"
)

func SaveImage(c echo.Context, source string, dest string) (string, error) {

	file, err := c.FormFile(source)
	if err != nil {
		return "", err
	}
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	dirName := path.Join("files", dest)
	filename := path.Join(dirName, file.Filename)

	// Destination
	err = os.MkdirAll(dirName, 0755)
	if err != nil {
		return "", err
	}

	dst, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Copy
	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}

	return filename, nil
}
