package controllers

import (
	"context"
)

// Internal struct
type Sales struct {
	ctx context.Context
}

func SalesApp() *Sales {
	return &Sales{}
}

// func (s *Sales) Create(sales *models.Sale) echo.Map {

// 	var sales models.Sale

// 	if sales.CustomerID == "" || len(sales.Details) == 0 {
// 		return c.JSON(http.StatusBadRequest, echo.Map{"message": "please enter all required fields."})
// 	}

// 	sales.SubTotal = 0
// 	for x, details := range sales.Details {
// 		sales.Details[x].Total = details.Qty * details.UnitPrice
// 		sales.SubTotal += sales.Details[x].Total
// 	}

// 	sales.StaffID = config.LoggedInStaffID
// 	sales.GrandTotal = sales.SubTotal - sales.DiscountAmount

// 	if err := config.DB.Create(&sales).Error; err != nil {
// 		return c.JSON(http.StatusBadRequest, echo.Map{"message": "error creating record: " + err.Error()})
// 	}

// 	return c.JSON(http.StatusOK, echo.Map{"id": sales.ID})
// }

// func Delete(c echo.Context) error {
// 	if c.Get("Role").(float64) > config.ADMIN_ROLE {
// 		return c.JSON(http.StatusUnauthorized, echo.Map{"message": "unauthorized access"})
// 	}

// 	invoice := &models.Invoice{}
// 	invoice.ID = c.Param("id")
// 	config.DB.Model(&models.Invoice{}).Preload("Payments").First(&invoice)

// 	if invoice.PatientID == "" {
// 		return c.JSON(http.StatusNotFound, echo.Map{"message": "record not found"})
// 	}

// 	config.DB.Delete(invoice)

// 	return c.JSON(http.StatusOK, echo.Map{"message": "record deleted successfully"})
// }

// func ReadAll(c echo.Context) error {
// 	var invoices []models.Invoice

// 	db := config.DB.Model(&models.Invoice{})

// 	if balance := c.QueryParam("balance"); balance == "true" {
// 		db.Where("balance > 0")
// 	} else if balance == "false" {
// 		db.Where("balance == 0")
// 	} else {
// 		db.Order("balance DESC, created_at ASC")
// 	}

// 	if patient_id := c.QueryParam("patient_id"); patient_id != "" {
// 		db.Where("patient_id = '" + patient_id + "'")
// 	}

// 	db.Preload("Patient").Find(&invoices)

// 	return c.JSON(http.StatusOK, invoices)
// }
