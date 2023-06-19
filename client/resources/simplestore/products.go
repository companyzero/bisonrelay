package simplestore

import (
	"fmt"
	"strconv"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/decred/dcrd/dcrutil/v4"
)

type Product struct {
	Title       string   `json:"title"`
	SKU         string   `json:"sku"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Price       float64  `json:"price"`
	Disabled    bool     `json:"disabled,omitempty"`
	Shipping    bool     `json:"shipping"`
}

type productsFile struct {
	Products []*Product
}

type CartItem struct {
	Product  *Product `json:"product"`
	Quantity uint32   `json:"quantity"`
}

type Cart struct {
	Items   []*CartItem `json:"items"`
	Updated time.Time   `json:"updated"`
}

// Total returns the total amount, with 2 decimal places accuracy.
func (cart *Cart) TotalCents() int64 {
	var totalUSDCents int64
	for _, item := range cart.Items {
		totalItemUSDCents := int64(item.Quantity) * int64(item.Product.Price*100)
		totalUSDCents += totalItemUSDCents
	}
	return totalUSDCents
}

// Total returns the total cart amount in USD.
func (cart *Cart) Total() float64 {
	return float64(cart.TotalCents()) / 100
}

type OrderID uint32

func (id OrderID) String() string {
	return fmt.Sprintf("%08d", id)
}

func (id *OrderID) FromString(s string) error {
	i, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return err
	}
	*id = OrderID(i)
	return nil
}

type OrderStatus string

const (
	StatusPlaced    OrderStatus = "placed"
	StatusShipped   OrderStatus = "shipped"
	StatusCompleted OrderStatus = "completed"
	StatusCanceled  OrderStatus = "canceled"
)

type ShippingAddress struct {
	Name        string `json:"name"`
	Address1    string `json:"address1"`
	Address2    string `json:"address2"`
	City        string `json:"city"`
	State       string `json:"state"`
	PostalCode  string `json:"postalCode"`
	Phone       string `json:"phone"`
	CountryCode string `json:"countrycode"`
}

type OrderComment struct {
	Timestamp time.Time `json:"ts"`
	FromAdmin bool      `json:"fromAdmin"`
	Comment   string    `json:"comment"`
}

type Order struct {
	ID           OrderID           `json:"id"`
	User         clientintf.UserID `json:"user"`
	Cart         Cart              `json:"cart"`
	Status       OrderStatus       `json:"status"`
	PlacedTS     time.Time         `json:"placed_ts"`
	ResolvedTS   *time.Time        `json:"resolved_ts"`
	ShipCharge   float64           `json:"ship_charge"`
	ExchangeRate float64           `json:"exchange_rate"`
	PayType      PayType           `json:"pay_type"`
	Invoice      string            `json:"invoice"`
	ShipAddr     *ShippingAddress  `json:"shipping"`
	Comments     []OrderComment    `json:"comments"`
}

// Total returns the total amount, with 2 decimal places accuracy.
func (order *Order) TotalCents() int64 {
	totalUSDCents := order.Cart.TotalCents()
	if order.ShipCharge > 0 {
		totalUSDCents += int64(order.ShipCharge * 100)
	}
	return totalUSDCents
}

// Total returns the total amount as a float USD.
func (order *Order) Total() float64 {
	return float64(order.TotalCents()) / 100
}

// TotalDCR returns the total order amount in DCR, given the configured exchange
// rate.
func (order *Order) TotalDCR() dcrutil.Amount {
	if order.ExchangeRate == 0 {
		return 0
	}
	totalDCR := order.Total() / order.ExchangeRate
	amount, _ := dcrutil.NewAmount(totalDCR)
	return amount
}
