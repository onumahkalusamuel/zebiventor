package models

import (
	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Sale struct {
	BaseModel
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"-"`
	StaffID         string           `gorm:"not null;references:staffs(id)" json:"-"`
	CustomerID      string           `gorm:"not null;references:customers(id)" json:"customer_id"`
	Details         []InvoiceDetails `gorm:"types:bytes;serializer:gob" json:"details"`
	SubTotal        uint             `gorm:"not 0" json:"sub_total"`
	DiscountAmount  uint             `gorm:"default 0" json:"discount_amount"`
	GrandTotal      uint             `gorm:"default 0" json:"grand_total"`
	PaymentMethod   string           `gorm:"default null" json:"payment_method"`
	PaymentRefernce string           `gorm:"default null" json:"payment_reference"`
	Customer        *Customer        `json:"customer"`
}

type InvoiceDetails struct {
	ProductName string `json:"product_name"`
	ProductID   string `json:"product_id"`
	ProductCode string `json:"product_code"`
	UnitPrice   uint   `json:"unit_price"`
	Qty         uint   `json:"qty"`
	Total       uint   `json:"total"`
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
