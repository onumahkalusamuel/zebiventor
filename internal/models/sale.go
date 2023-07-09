package models

import (
	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Sale struct {
	BaseModel
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"-"`
	StaffID         string           `gorm:"not null;references:staffs(id)" json:"-"`
	CustomerID      string           `gorm:"not null;references:customers(id)"`
	Details         []InvoiceDetails `gorm:"types:bytes;serializer:gob"`
	SubTotal        uint             `gorm:"not 0"`
	DiscountAmount  uint             `gorm:"default 0"`
	GrandTotal      uint             `gorm:"default 0"`
	PaymentRefernce string           `gorm:"default null"`
	Customer        *Customer
}

type InvoiceDetails struct {
	ProductName string
	ProductID   string
	UnitPrice   uint
	Qty         uint
	Total       uint
}

func (m *Sale) Update() error {
	return config.DB.First(&m, &m).Save(&m).Error
}

func (m *Sale) Delete() bool {
	if result := config.DB.First(&m, &m); result.Error != nil {
		return false
	}
	config.DB.Delete(&m)
	return true
}

func (m *Sale) Read() error {

	result := config.DB.First(&m, &m)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (m *Sale) ReadAll() (bool, []Sale) {
	var Sales []Sale
	if result := config.DB.Find(&Sales, &m); result.Error != nil {
		return false, Sales
	}
	return true, Sales
}
