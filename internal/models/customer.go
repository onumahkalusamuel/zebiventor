package models

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Customer struct {
	BaseModel
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	CustomerCode string         `gorm:"default:null;unique"`
	Name         string         `gorm:"default:null"`
	Phone        string         `gorm:"default:null"`
	Email        string         `gorm:"default:null"`
	Sex          string         `gorm:"default:null"`
	Sales        []*Sale        `gorm:"constraint:onDelete:CASCADE"`
}

func (m *Customer) Create() error {
	return config.DB.Create(&m).Error
}

func (m *Customer) Update() error {
	return config.DB.First(&m, &m).Save(&m).Error
}

func (m *Customer) UpdateSingle(key string, value any) error {
	return config.DB.First(&m).Update(key, value).Error
}

func (m *Customer) Delete() bool {
	if result := config.DB.First(&m, &m); result.Error != nil {
		return false
	}
	config.DB.Delete(&m)
	return true
}

func (m *Customer) Read() error {
	result := config.DB.First(&m, &m)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (m *Customer) ReadAll() (bool, []Customer) {
	var Customers []Customer
	if result := config.DB.Find(&Customers, &m); result.Error != nil {
		return false, Customers
	}
	return true, Customers
}

func (u *Customer) AfterCreate(tx *gorm.DB) (err error) {
	s := Settings{Setting: "last_card_no"}
	s.Read()
	lastNo, _ := strconv.Atoi(s.Value)
	available := false
	nextCardNo := ""

	for available == false {
		nextCardNo = strings.TrimLeft(fmt.Sprintf("H%04v", lastNo+1), " ")

		// check existing
		var check []Customer
		tx.Model(u).Find(&check, "card_no='"+nextCardNo+"'")
		if len(check) > 0 {
			lastNo++
			continue
		}

		available = true
		tx.Model(u).Update("card_no", nextCardNo)
		tx.Model(s).Update("value", lastNo+1)
	}

	return
}
