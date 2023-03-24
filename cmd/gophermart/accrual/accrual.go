package accrual

import (
	"encoding/json"
	conf "github.com/N0rkton/gophermart/internal/config"
	"github.com/N0rkton/gophermart/internal/datamodels"
	"github.com/N0rkton/gophermart/internal/storage"
	"io"
	"log"
	"net/http"
	"time"
)

type Wrapper struct {
	DB storage.Storage
}

func (aw Wrapper) Accrual() {
	allOrders, err := aw.DB.GetAllOrdersForAccrual()
	if err != nil {
		log.Println(err)
		return
	}
	if allOrders == nil {
		return
	}
	addr := conf.GetAccrualAddress()
	for _, v := range allOrders {
		url := addr + "/api/orders/" + v
		resp, err := http.Get(url)
		if err != nil {
			log.Println(err)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			payload, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
				continue
			}
			var accrual datamodels.Accrual
			err = json.Unmarshal(payload, &accrual)
			if err != nil {
				log.Println(err)
				continue
			}
			aw.DB.UpdateAccrual(accrual)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(3 * time.Second)
		}
	}
}
