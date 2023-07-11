package models

import (
	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Product struct {
	BaseModel
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	CategoryID    string         `gorm:"not null;references:categories(id)" json:"category_id"`
	Code          string         `gorm:"default:null;unique" json:"code"`
	Name          string         `gorm:"default:null" json:"name"`
	Price         uint           `gorm:"default:null" json:"price"`
	StockQuantity uint           `gorm:"default:null" json:"stock_quantity"`
	Category      *Category      `json:"category"`
}

func (m *Product) Create() error {
	return config.DB.Create(&m).Error
}

func (m *Product) Update() error {
	return config.DB.First(&m, &m).Save(&m).Error
}

func (m *Product) UpdateSingle(key string, value any) error {
	return config.DB.First(&m).Update(key, value).Error
}

func (m *Product) Delete() bool {
	if result := config.DB.First(&m, &m); result.Error != nil {
		return false
	}
	config.DB.Delete(&m)
	return true
}

func (m *Product) Read() error {
	result := config.DB.First(&m, &m)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (m *Product) ReadAll() (bool, []Product) {
	var Products []Product
	if result := config.DB.Find(&Products, &m); result.Error != nil {
		return false, Products
	}
	return true, Products
}
