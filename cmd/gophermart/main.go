package main

import (
	"fmt"
	"github.com/N0rkton/gophermart/internal/config"
	"github.com/N0rkton/gophermart/internal/handlers"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func main() {
	handlers.Init()
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			handlers.Accrual()
			fmt.Println("ticker")
		}
	}()
	router := mux.NewRouter()
	router.HandleFunc("/api/user/register", handlers.Register).Methods(http.MethodPost)
	router.HandleFunc("/api/user/login", handlers.Login).Methods(http.MethodPost)
	router.HandleFunc("/api/user/orders", handlers.OrdersPost).Methods(http.MethodPost)
	router.HandleFunc("/api/user/balance/withdraw", handlers.Withdraw).Methods(http.MethodPost)

	router.HandleFunc("/api/user/orders", handlers.OrdersGet).Methods(http.MethodGet)
	router.HandleFunc("/api/user/balance", handlers.Balance).Methods(http.MethodGet)
	router.HandleFunc("/api/user/withdrawals", handlers.Withdrawals).Methods(http.MethodGet)

	log.Fatal(http.ListenAndServe(config.GetServerAddress(), handlers.GzipHandle(router)))

}
