package helpers

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"

	gomail "gopkg.in/mail.v2"
)

func SendEmail(sender string, recipient string, subject string, message string) error {

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("MAILER_FROM"))
	m.SetHeader("To", recipient)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", message)

	// Settings for SMTP server
	port, err := strconv.Atoi(os.Getenv("MAILER_PORT"))
	if err != nil {
		return err
	}

	d := gomail.NewDialer(os.Getenv("MAILER_HOST"), port, os.Getenv("MAILER_FROM"), os.Getenv("MAILER_FROM_PASSWORD"))

	d.TLSConfig = &tls.Config{InsecureSkipVerify: os.Getenv("ENV") == "dev"}

	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}
