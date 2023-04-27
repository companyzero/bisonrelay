package simplestore

import (
	"strconv"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
)

type Product struct {
	Title       string   `json:"title"`
	SKU         string   `json:"sku"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Price       float64  `json:"price"`
	Disabled    bool     `json:"disabled,omitempty"`
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

type OrderID uint32

func (id OrderID) String() string {
	return strconv.FormatUint(uint64(id), 10)
}

type Order struct {
	ID   OrderID           `json:"id"`
	User clientintf.UserID `json:"user"`
	Cart Cart              `json:"cart"`
}

// Total returns the total amount, with 2 decimal places accuracy.
func (order *Order) TotalCents() int64 {
	var totalUSDCents int64
	for _, item := range order.Cart.Items {
		totalItemUSDCents := int64(item.Quantity) * int64(item.Product.Price*100)
		totalUSDCents += totalItemUSDCents
	}
	return totalUSDCents
}
