package models

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Product struct {
	BaseModel
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	CategoryID    string         `gorm:"not null;references:categories(id)"`
	SKU           string         `gorm:"default:null;unique"`
	Name          string         `gorm:"default:null"`
	Price         uint           `gorm:"default:null"`
	StockQuantity string         `gorm:"default:null"`
	Category      *Category
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

func (u *Product) AfterCreate(tx *gorm.DB) (err error) {
	s := Settings{Setting: "last_card_no"}
	s.Read()
	lastNo, _ := strconv.Atoi(s.Value)
	available := false
	nextCardNo := ""

	for available == false {
		nextCardNo = strings.TrimLeft(fmt.Sprintf("H%04v", lastNo+1), " ")

		// check existing
		var check []Product
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
