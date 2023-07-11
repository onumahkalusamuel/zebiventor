package controllers

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/helpers"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
)

// Internal struct
type Product struct {
	ctx context.Context
}

func ProductApp() *Product {
	return &Product{}
}

func (s *Product) Create(product *models.Product) map[string]interface{} {

	if product.Name == "" {
		return echo.Map{
			"success": false,
			"message": "please enter a name for product fields.",
		}
	}

	// check code
	var inuse = []models.Product{}
	config.DB.Unscoped().Where("code='" + product.Code + "'").Find(&inuse)

	if len(inuse) > 0 || product.Code == "" {
		var gotten = false
		for gotten == false {
			product.Code = helpers.String(4)
			var secondUse = []models.Product{}
			config.DB.Unscoped().Where("code='" + product.Code + "'").Find(&inuse)
			if len(secondUse) == 0 {
				gotten = true
				break
			}
		}
	}

	if err := config.DB.Create(&product).Error; err != nil {
		return echo.Map{
			"success": false,
			"message": "error adding product. " + err.Error(),
		}
	}

	return echo.Map{"success": true, "id": product.ID}
}

func (s *Product) Delete(id string) map[string]interface{} {
	// if config.LoggedIn.Role > config.ADMIN_ROLE {
	// 	return echo.Map{"message": "unauthorized access"}
	// }

	product := &models.Product{}
	product.ID = id
	product.Read()

	if product.Name == "" {
		return echo.Map{
			"success": false,
			"message": "record not found",
		}
	}

	product.Delete()

	return echo.Map{
		"success": true,
		"message": "product deleted successfully",
	}
}

func (s *Product) ReadOne(id string) *models.Product {
	product := &models.Product{}
	product.ID = id
	product.Read()
	return product
}

func (s *Product) FindByCode(code string) *models.Product {
	product := &models.Product{}
	product.Code = code
	product.Read()
	return product
}

func (s *Product) Update(id string, req *models.Product) map[string]interface{} {

	product := &models.Product{}
	product.ID = id
	product.Read()

	if err := product.Read(); err != nil {
		return echo.Map{
			"success": false,
			"message": "bad request: " + err.Error()}
	}

	if req.Name != product.Name {
		product.UpdateSingle("name", req.Name)
	}

	return echo.Map{
		"success": true,
		"message": "record updated successfully.",
	}
}

type ProductSearchParams struct {
	Query      string `json:"query"`
	CategoryID string `json:"category_id"`
}

func (s *Product) ReadAll(params *ProductSearchParams) []models.Product {

	var products []models.Product

	db := config.DB.Model(&models.Product{})

	if params.Query != "" {
		db.Where("name LIKE '%" + params.Query + "%' OR code LIKE '%" + params.Query + "%'")
	}

	if params.CategoryID != "" {
		db.Where("category_id = '" + params.CategoryID + "'")
	}

	db.Order("name ASC, created_at ASC")

	db.Preload("Category").Find(&products)

	return products
}
