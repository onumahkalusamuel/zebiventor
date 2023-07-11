package controllers

import (
	"context"

	"github.com/labstack/echo"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
)

// Internal struct
type Sales struct {
	ctx context.Context
}

func SalesApp() *Sales {
	return &Sales{}
}

func (s *Sales) Create(sales *models.Sale) map[string]interface{} {

	if sales.CustomerID == "" || len(sales.Details) == 0 {
		return echo.Map{
			"success": false,
			"message": "please enter all required fields.",
		}
	}

	sales.SubTotal = 0
	for x, details := range sales.Details {
		sales.Details[x].Total = details.Qty * details.UnitPrice
		sales.SubTotal += sales.Details[x].Total
	}

	sales.StaffID = config.LoggedIn.ID
	sales.GrandTotal = sales.SubTotal - sales.DiscountAmount

	if err := config.DB.Create(&sales).Error; err != nil {
		return echo.Map{
			"success": false,
			"message": "error creating record: " + err.Error(),
		}
	}

	// decrease stock
	for _, details := range sales.Details {

		product := &models.Product{}
		product.Code = details.ProductCode
		config.DB.Model(&models.Product{}).First(&product)

		if product.ID == "" {
			continue
		}

		product.StockQuantity = product.StockQuantity - details.Qty

		config.DB.Updates(product)
	}

	return echo.Map{
		"success": true,
		"id":      sales.ID,
	}
}

func (s *Sales) Delete(id string) map[string]interface{} {
	if config.LoggedIn.Role > config.ADMIN_ROLE {
		return echo.Map{
			"success": false,
			"message": "unauthorized access",
		}
	}

	sale := &models.Sale{}
	sale.ID = id
	config.DB.Model(&models.Sale{}).First(&sale)

	if sale.CustomerID == "" {
		return echo.Map{
			"success": false,
			"message": "record not found",
		}
	}

	config.DB.Delete(sale)

	return echo.Map{
		"success": true,
		"message": "record deleted successfully",
	}
}

type SalesSearch struct {
	CustomerID string `json:"customer_id"`
	CreatedAt  string `json:"created_at"`
}

func (s *Sales) ReadAll(searchParam *SalesSearch) []models.Sale {
	var sales []models.Sale

	db := config.DB.Model(&models.Sale{})

	// if balance := c.QueryParam("balance"); balance == "true" {
	// 	db.Where("balance > 0")
	// } else if balance == "false" {
	// 	db.Where("balance == 0")
	// } else {
	// 	db.Order("balance DESC, created_at ASC")
	// }

	if searchParam.CustomerID != "" {
		db.Where("customer_id = '" + searchParam.CustomerID + "'")
	}

	db.Preload("Customer").Find(&sales)

	return sales
}

func (s *Sales) ReadOne(id string) *models.Sale {
	sale := &models.Sale{}
	sale.ID = id
	db := config.DB.Model(&models.Sale{})
	db.Preload("Customer").First(&sale)
	return sale
}

func ProcessStockDeductions() {}
