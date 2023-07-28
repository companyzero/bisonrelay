package simplestore

import (
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
)

type indexContext struct {
	Products map[string]*Product
	IsAdmin  bool
}

type addToCartContext struct {
	Product *Product
	Cart    *Cart
}

type orderContext struct {
	Order
}

type ordersContext struct {
	Orders []*Order
}

type adminOrderSummary struct {
	ID       OrderID
	User     clientintf.UserID
	UserNick string
	Status   OrderStatus
	PlacedTS time.Time
}

type adminOrdersContext struct {
	Orders []adminOrderSummary
}

type adminOrderContext struct {
	Order    Order
	UserNick string
}
