package controllers

import "context"

// Internal struct
type Customers struct {
	ctx context.Context
}

func CustomersApp() *Customers {
	return &Customers{}
}
