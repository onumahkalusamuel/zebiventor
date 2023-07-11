package models

import (
	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Category struct {
	BaseModel
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"default:null" json:"name"`
	Products  []*Product     `gorm:"constraint:onDelete:SET NULL" json:"products"`
}

func (m *Category) Create() error {
	return config.DB.Create(&m).Error
}

func (m *Category) Update() error {
	return config.DB.First(&m, &m).Save(&m).Error
}

func (m *Category) UpdateSingle(key string, value any) error {
	return config.DB.First(&m).Update(key, value).Error
}

func (m *Category) Delete() bool {
	if result := config.DB.First(&m, &m); result.Error != nil {
		return false
	}
	config.DB.Delete(&m)
	return true
}

func (m *Category) Read() error {
	result := config.DB.First(&m, &m)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (m *Category) ReadAll() (bool, []Category) {
	var Categorys []Category
	if result := config.DB.Find(&Categorys, &m); result.Error != nil {
		return false, Categorys
	}
	return true, Categorys
}
