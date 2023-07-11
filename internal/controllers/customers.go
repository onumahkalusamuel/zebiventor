package controllers

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/helpers"
	"github.com/onumahkalusamuel/zebiventor/internal/models"
)

// Internal struct
type Customer struct {
	ctx context.Context
}

func CustomersApp() *Customer {
	return &Customer{}
}

func (s *Customer) Create(customer *models.Customer) map[string]interface{} {

	if customer.Name == "" {
		return echo.Map{
			"success": false,
			"message": "please enter a name for customer fields.",
		}
	}

	// check code
	var codeInUse = []models.Customer{}
	config.DB.Unscoped().Where("code='" + customer.CustomerCode + "'").Find(&codeInUse)

	if len(codeInUse) > 0 || customer.CustomerCode == "" {
		var gotten = false
		for gotten == false {
			customer.CustomerCode = helpers.String(4)
			var secondUse = []models.Customer{}
			config.DB.Unscoped().Where("code='" + customer.CustomerCode + "'").Find(&codeInUse)
			if len(secondUse) == 0 {
				gotten = true
				break
			}
		}
	}

	// check name
	var inuse = []models.Customer{}
	config.DB.Unscoped().Where("name='" + customer.Name + "'").Find(&inuse)

	if len(inuse) > 0 {
		return echo.Map{
			"success": false,
			"message": "Customer name already in use.",
		}
	}

	if err := config.DB.Create(&customer).Error; err != nil {
		return echo.Map{
			"success": false,
			"message": "error adding customer. " + err.Error(),
		}
	}

	return echo.Map{"success": true, "id": customer.ID}
}

func (s *Customer) Delete(id string) map[string]interface{} {
	customer := &models.Customer{}
	customer.ID = id
	customer.Read()

	if customer.Name == "" {
		return echo.Map{
			"success": false,
			"message": "record not found",
		}
	}

	customer.Delete()

	return echo.Map{
		"success": true,
		"message": "customer deleted successfully",
	}
}

func (s *Customer) ReadOne(id string) *models.Customer {
	customer := &models.Customer{}
	customer.ID = id
	db := config.DB.Model(&models.Customer{})
	db.Preload("Sales").First(&customer)
	return customer
}

func (s *Customer) Update(id string, name string) map[string]interface{} {

	customer := &models.Customer{}
	customer.ID = id
	customer.Read()

	if err := customer.Read(); err != nil {
		return echo.Map{
			"success": false,
			"message": "bad request: " + err.Error()}
	}

	if name != customer.Name {
		customer.UpdateSingle("name", name)
	}

	return echo.Map{
		"success": true,
		"message": "record updated successfully.",
	}
}

type CustomerSearchParams struct {
	Query string `json:"query"`
	ID    string `json:"customer_id"`
}

func (s *Customer) ReadAll(params *CustomerSearchParams) []models.Customer {

	var products []models.Customer

	db := config.DB.Model(&models.Customer{})

	if params.Query != "" {
		db.Where("name LIKE '%" + params.Query + "%' OR customer_code LIKE '%" + params.Query + "%'")
	}

	if params.ID != "" {
		db.Where("id = '" + params.ID + "'")
	}

	db.Order("customer_code ASC, created_at ASC")

	db.Find(&products)

	return products
}
