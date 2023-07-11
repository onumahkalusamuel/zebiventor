package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        string    `gorm:"primarykey" json:"id"`
	UpdatedAt time.Time `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *BaseModel) BeforeCreate(tx *gorm.DB) (err error) {
	UUID, _ := uuid.NewRandom()
	u.ID = UUID.String()
	return
}
