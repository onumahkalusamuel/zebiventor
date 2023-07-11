package models

import (
	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Customer struct {
	BaseModel
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	CustomerCode string         `gorm:"default:null;unique" json:"customer_code"`
	Name         string         `gorm:"default:null" json:"name"`
	Phone        string         `gorm:"default:null" json:"phone"`
	Email        string         `gorm:"default:null" json:"email"`
	Sex          string         `gorm:"default:null" json:"sex"`
	Sales        []*Sale        `gorm:"constraint:onDelete:CASCADE" json:"sales"`
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
