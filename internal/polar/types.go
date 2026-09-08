package polar

type Price struct {
	ID            string `json:"id"`
	AmountType    string `json:"amount_type"`
	PriceAmount   int    `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
}

type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsArchived  bool    `json:"is_archived"`
	Prices      []Price `json:"prices"`
	CreatedAt   string  `json:"created_at"`
}

type ProductList struct {
	Items      []Product  `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	TotalCount int `json:"total_count"`
	MaxPage    int `json:"max_page"`
}

type CheckoutRequest struct {
	Products      []string          `json:"products"`
	CustomerEmail string            `json:"customer_email,omitempty"`
	CustomerName  string            `json:"customer_name,omitempty"`
	SuccessURL    string            `json:"success_url,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type Checkout struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	URL          string `json:"url"`
	ClientSecret string `json:"client_secret"`
	TotalAmount  int    `json:"total_amount"`
	Currency     string `json:"currency"`
	CreatedAt    string `json:"created_at"`
}

type OrderCustomer struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Order struct {
	ID          string         `json:"id"`
	Status      string         `json:"status"`
	Amount      int            `json:"amount"`
	Currency    string         `json:"currency"`
	Customer    *OrderCustomer `json:"customer"`
	ProductID   string         `json:"product_id"`
	Product     *Product       `json:"product"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"created_at"`
}

type OrderList struct {
	Items      []Order    `json:"items"`
	Pagination Pagination `json:"pagination"`
}
