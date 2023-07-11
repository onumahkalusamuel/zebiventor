package db

import (
	"log"

	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init() {

	var err error
	config.DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

	if err != nil {
		log.Fatalln(err)
	}

	config.DB.AutoMigrate(
		&models.Staff{},
		&models.Customer{},
		&models.Category{},
		&models.Product{},
		&models.Sale{},
		&models.Settings{},
	)
}

func InitialData() {
	for setting, value := range config.InitSettings {
		s := &models.Settings{Setting: setting}
		s.Read()
		if s.ID == "" {
			s = &models.Settings{Setting: setting, Value: value}
			s.Create()
		}
	}

	c := &models.Customer{}

	config.DB.Find(&c).First(&c)
	if c.ID == "" {
		c = &models.Customer{
			Name:         "Guest User",
			Email:        "quest@guest.com",
			CustomerCode: "GUEST",
		}
		config.DB.Create(&c)
	}
}
