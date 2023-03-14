package datamodels

import "time"

type Auth struct {
	ID       int
	Password string
}
type Order struct {
	OrderID     string    `json:"order_id"`
	OrderStatus string    `json:"order_status"`
	Accrual     float32   `json:"accrual,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
type Withdrawals struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}
type OrderInfo struct {
	UserID  int
	OrderID int
	Sum     float64
}
type Reg struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
type Withdraw struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}
type Accrual struct {
	Order   string `json:"order"`
	Status  string `json:"status"`
	Accrual int    `json:"accrual"`
}
