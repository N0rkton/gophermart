package accrualclient

import (
	"encoding/json"
	"errors"
	conf "github.com/N0rkton/gophermart/internal/config"
	"io"
	"log"
	"net/http"
	"time"
)

type AccrualClient interface {
	GetOrder(orderNumber string) (Order, error)
}
type Order struct {
	OrderID string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float32 `json:"accrual"`
}
type accrualClient struct {
	accrualAddr string // -> http://accrualdomain.com/api/orders
}

func NewAC() AccrualClient {
	return &accrualClient{accrualAddr: conf.GetAccrualAddress()}
}
func (ac *accrualClient) GetOrder(orderNumber string) (Order, error) {

	url := ac.accrualAddr + "/api/orders/" + orderNumber
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return Order{}, err
	}
	if resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			return Order{}, err
		}
		var tmp Order
		err = json.Unmarshal(payload, &tmp)
		if err != nil {
			log.Println(err)
			return Order{}, err
		}
		return tmp, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		time.Sleep(3 * time.Second)
	}
	return Order{}, errors.New("internal error")
}
