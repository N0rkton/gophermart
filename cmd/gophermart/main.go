package main

import (
	"github.com/N0rkton/gophermart/cmd/gophermart/accrualClient"
	"github.com/N0rkton/gophermart/internal/config"
	"github.com/N0rkton/gophermart/internal/datamodels"
	"github.com/N0rkton/gophermart/internal/handlers"
	"github.com/N0rkton/gophermart/internal/storage"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func main() {
	ws := handlers.Init()

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			orders(ws.DB)
		}
	}()
	router := mux.NewRouter()
	router.HandleFunc("/api/user/register", ws.Register).Methods(http.MethodPost)
	router.HandleFunc("/api/user/login", ws.Login).Methods(http.MethodPost)
	router.HandleFunc("/api/user/orders", ws.OrdersPost).Methods(http.MethodPost)
	router.HandleFunc("/api/user/balance/withdraw", ws.Withdraw).Methods(http.MethodPost)

	router.HandleFunc("/api/user/orders", ws.OrdersGet).Methods(http.MethodGet)
	router.HandleFunc("/api/user/balance", ws.Balance).Methods(http.MethodGet)
	router.HandleFunc("/api/user/withdrawals", ws.Withdrawals).Methods(http.MethodGet)

	log.Fatal(http.ListenAndServe(config.GetServerAddress(), ws.GzipHandle(router)))

}
func orders(db storage.Storage) {
	allOrders, err := db.GetAllOrdersForAccrual()
	if err != nil {
		log.Println(err)
		return
	}
	if allOrders == nil {
		return
	}
	for _, v := range allOrders {
		var order accrualClient.Order
		order, err = order.GetOrder(v)
		if err == nil {
			db.UpdateAccrual(datamodels.Accrual{Order: order.OrderId, Accrual: order.Accrual, Status: order.Status})
		}
	}
}
