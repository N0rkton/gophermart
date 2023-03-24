package main

import (
	"github.com/N0rkton/gophermart/cmd/gophermart/accrual"
	"github.com/N0rkton/gophermart/internal/config"
	"github.com/N0rkton/gophermart/internal/handlers"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func main() {
	ws := handlers.Init()
	as := accrual.Wrapper{ws.DB}
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			as.Accrual()
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
