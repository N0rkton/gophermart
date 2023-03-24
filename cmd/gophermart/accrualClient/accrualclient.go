package accrualClient

import (
	"encoding/json"
	"errors"
	conf "github.com/N0rkton/gophermart/internal/config"
	"github.com/N0rkton/gophermart/internal/datamodels"
	"io"
	"log"
	"net/http"
	"time"
)

type AccrualClient interface {
	GetOrder(v string) (datamodels.Accrual, error)
}
type Order struct {
	OrderId string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float32 `json:"accrual"`
}

func (ac Order) GetOrder(v string) (Order, error) {

	addr := conf.GetAccrualAddress()
	url := addr + "/api/orders/" + v
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

		err = json.Unmarshal(payload, &ac)
		if err != nil {
			log.Println(err)
			return Order{}, err
		}
		return ac, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		time.Sleep(3 * time.Second)
	}
	return Order{}, errors.New("internal error")
}
