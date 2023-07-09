package models

import (
	"github.com/onumahkalusamuel/zebiventor/config"
)

// Settings struct
type Settings struct {
	BaseModel
	Setting string `gorm:"not null;uniqueIndex;unique"`
	Value   string `gorm:"not null"`
}

// Create creates a new setting
func (s *Settings) Create() error {
	return config.DB.Create(&s).Error
}

// Read reads a setting
func (s *Settings) Read() error {
	return config.DB.First(&s, &s).Error
}

// Update updates a setting
func (s *Settings) Update() error {
	return config.DB.Save(s).Error
}

// create Update function
func (m *Settings) UpdateSingle(key string, value string) error {
	return config.DB.First(&m).Update(key, value).Error
}

// Delete deletes a setting
func (s *Settings) Delete() bool {
	if result := config.DB.First(&s, &s); result.Error != nil {
		return false
	}
	config.DB.Delete(&s)
	return true
}

// ReadAll reads all settings
func (s *Settings) ReadAll() (bool, []Settings) {
	var settings []Settings
	if result := config.DB.Find(&settings, &s); result.Error != nil {
		return false, settings
	}
	return true, settings
}
