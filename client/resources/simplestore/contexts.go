package simplestore

type indexContext struct {
	Products map[string]*Product
}

type addToCartContext struct {
	Product *Product
	Cart    *Cart
}

type ordersContext struct {
	Orders []*Order
}
