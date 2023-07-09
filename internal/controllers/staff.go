package controllers

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
	"github.com/onumahkalusamuel/zebiventor/pkg"
)

// Internal struct
type Staff struct {
	ctx context.Context
}

func StaffApp() *Staff {
	return &Staff{}
}

type CreateStaffRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	Role     uint   `json:"role"`
	Sex      string `json:"sex"`
}

func (s *Staff) Create(staff *CreateStaffRequest) map[string]interface{} {

	if config.LoggedIn.Role > config.ADMIN_ROLE {
		return echo.Map{"message": "unauthorized access"}
	}

	if staff.Name == "" || staff.Username == "" || staff.Password == "" {
		return echo.Map{"message": "please enter all required fields."}
	}

	// check username
	var inuse = []models.Staff{}
	config.DB.Unscoped().Where("username='" + staff.Username + "'").Find(&inuse)

	if len(inuse) > 0 {
		return echo.Map{"message": "username already in use"}
	}

	// hash password
	staff.Password = pkg.HashPassword(staff.Password)

	newstaff := models.Staff{
		Name:     staff.Name,
		Username: staff.Username,
		Phone:    staff.Phone,
		Password: staff.Password,
		Role:     staff.Role,
		Sex:      staff.Sex,
	}

	if err := config.DB.Create(&newstaff).Error; err != nil {
		return echo.Map{"message": "error creating account: " + err.Error()}
	}

	return echo.Map{"id": newstaff.ID}
}

func (s *Staff) Delete(id string) map[string]interface{} {
	if config.LoggedIn.Role > config.ADMIN_ROLE {
		return echo.Map{"message": "unauthorized access"}
	}

	staff := &models.Staff{}
	staff.ID = id
	staff.Read()

	if staff.Role == config.ADMIN_ROLE {
		return echo.Map{"message": "Cannot delete the Super Admin"}
	}

	if staff.Name == "" {
		return echo.Map{"message": "account not found"}
	}

	staff.Delete()

	return echo.Map{"message": "account deleted successfully"}
}

func (s *Staff) ReadOne(id string) *models.Staff {
	staff := &models.Staff{}
	staff.ID = id
	staff.Read()
	return staff
}

func (s *Staff) ReadProfile() *models.Staff {
	staff := &models.Staff{}
	staff.ID = config.LoggedIn.ID
	staff.Read()
	return staff
}

// update
type UpdateStaffRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
}

func (s *Staff) Update(id string, req *UpdateStaffRequest) map[string]interface{} {
	if config.LoggedIn.Role > config.ADMIN_ROLE {
		return echo.Map{"message": "unauthorized access"}
	}

	staff := &models.Staff{}
	staff.ID = id
	staff.Read()

	if err := staff.Read(); err != nil {
		return echo.Map{"message": "bad request: " + err.Error()}
	}

	if req.Name != staff.Name {
		staff.UpdateSingle("name", req.Name)
	}

	if req.Password != "" {
		staff.UpdateSingle("password", pkg.HashPassword(req.Password))
	}

	return echo.Map{"message": "record updated successfully."}
}

// update password
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Staff) UpdatePassword(req *UpdatePasswordRequest) map[string]interface{} {

	staff := &models.Staff{}
	staff.ID = config.LoggedIn.ID
	staff.Read()

	if err := staff.Read(); err != nil {
		return echo.Map{"message": "bad request: " + err.Error()}
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return echo.Map{"message": "enter old and new password."}
	}

	if pkg.CheckPassword(staff.Password, req.OldPassword) == false {
		return echo.Map{"message": "wrong old password provided."}
	}

	staff.Password = pkg.HashPassword(req.NewPassword)
	config.DB.Model(&models.Staff{}).Updates(staff)

	return echo.Map{"message": "password changed successfully."}
}
