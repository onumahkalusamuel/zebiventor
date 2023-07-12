package controllers

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
)

// Internal struct
type Category struct {
	ctx context.Context
}

func CategoryApp() *Category {
	return &Category{}
}

func (s *Category) Create(name string) map[string]interface{} {

	if name == "" {
		return echo.Map{
			"success": false,
			"message": "please enter a name for category fields.",
		}
	}

	// check name
	var inuse = []models.Category{}
	config.DB.Unscoped().Where("name='" + name + "'").Find(&inuse)

	if len(inuse) > 0 {
		return echo.Map{
			"success": false,
			"message": "Category name already in use.",
		}
	}

	var category models.Category
	category.Name = name
	if err := config.DB.Create(&category).Error; err != nil {
		return echo.Map{
			"success": false,
			"message": "error adding category. " + err.Error(),
		}
	}

	return echo.Map{"success": true, "id": category.ID}
}

func (s *Category) Delete(id string) map[string]interface{} {
	category := &models.Category{}
	category.ID = id
	db := config.DB.Model(&models.Category{})
	db.Preload("Products").Find(&category)

	if category.Name == "" {
		return echo.Map{
			"success": false,
			"message": "record not found",
		}
	}

	if len(category.Products) > 0 {
		return echo.Map{
			"success": false,
			"message": "Cannot delete category with products. Delete products first.",
		}
	}

	category.Delete()

	return echo.Map{
		"success": true,
		"message": "category deleted successfully",
	}
}

func (s *Category) ReadOne(id string) *models.Category {
	category := &models.Category{}
	category.ID = id
	category.Read()
	return category
}

func (s *Category) Update(id string, name string) map[string]interface{} {

	category := &models.Category{}
	category.ID = id
	category.Read()

	if err := category.Read(); err != nil {
		return echo.Map{
			"success": false,
			"message": "bad request: " + err.Error()}
	}

	if name != category.Name {
		category.UpdateSingle("name", name)
	}

	return echo.Map{
		"success": true,
		"message": "record updated successfully.",
	}
}

func (s *Category) ReadAll() []models.Category {
	var cats = models.Category{}
	_, all := cats.ReadAll()
	return all
}
