package source

const OrdersDir = "orders" // w/o trailing slash

type Source struct {
	Paths *Paths `json:"paths"`
	Zip   *Zip   `json:"zip"`
	Git   *Git   `json:"git"`
}
