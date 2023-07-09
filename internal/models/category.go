package models

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/config"
	"gorm.io/gorm"
)

type Category struct {
	BaseModel
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"default:null"`
	Price     uint           `gorm:"default:null"`
	Products  []*Product     `gorm:"constraint:onDelete:SET NULL"`
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

func (u *Category) AfterCreate(tx *gorm.DB) (err error) {
	s := Settings{Setting: "last_card_no"}
	s.Read()
	lastNo, _ := strconv.Atoi(s.Value)
	available := false
	nextCardNo := ""

	for available == false {
		nextCardNo = strings.TrimLeft(fmt.Sprintf("H%04v", lastNo+1), " ")

		// check existing
		var check []Category
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
