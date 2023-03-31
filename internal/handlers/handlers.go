package handlers

import (
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	conf "github.com/N0rkton/gophermart/internal/config"
	"github.com/N0rkton/gophermart/internal/cookies"
	"github.com/N0rkton/gophermart/internal/datamodels"
	"github.com/N0rkton/gophermart/internal/sessionstorage"
	"github.com/N0rkton/gophermart/internal/storage"
	"github.com/N0rkton/gophermart/internal/utils"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type wrapperStruct struct {
	DB        storage.Storage
	secret    []byte
	authUsers sessionstorage.SessionStorage
}

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
func gzipDecode(r *http.Request) io.ReadCloser {
	if r.Header.Get(`Content-Encoding`) == `gzip` {
		gz, _ := gzip.NewReader(r.Body)
		defer gz.Close()
		return gz
	}
	return r.Body
}

type contextKey int

const authenticatedUserKey contextKey = 0

func (ws wrapperStruct) GzipHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := cookies.ReadEncrypted(r, "UserID", ws.secret)
		if err != nil {
			user = "err"
		}
		ctxWithUser := context.WithValue(r.Context(), authenticatedUserKey, user)
		rWithUser := r.WithContext(ctxWithUser)
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, rWithUser)
			return
		}
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gz.Close()
		w.Header().Set("Content-Encoding", "gzip")
		rWithUser.Body = gzipDecode(r)
		next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, rWithUser)
	})
}

func Init() wrapperStruct {
	config := conf.NewConfig()
	var err error

	db, err := storage.NewDBStorage(*config.DBAddress)
	if err != nil {
		log.Println(err)
	}
	secret, err := hex.DecodeString("13d6b4dff8f84a10851021ec8608f814570d562c92fe6b5ec4c9f595bcb3234b")
	if err != nil {
		log.Fatal(err)
	}
	authUsers := sessionstorage.NewAuthUsersStorage()
	return wrapperStruct{DB: db, secret: secret, authUsers: authUsers}
}

func (ws wrapperStruct) Register(w http.ResponseWriter, r *http.Request) {
	var body datamodels.Reg
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	password := utils.GetMD5Hash(body.Password)
	fmt.Println(password)

	err = ws.DB.Register(body.Login, password)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		http.Error(w, "login already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "-", http.StatusBadRequest)
		return
	}
	id, err := ws.DB.Login(body.Login, password)
	if err != nil {
		http.Error(w, "server err", http.StatusInternalServerError)
	}
	user := utils.GenerateRandomString(3)
	cookie := http.Cookie{
		Name:     "UserID",
		Value:    user,
		Path:     "/api/user",
		HttpOnly: true,
		Secure:   false,
	}
	err = cookies.WriteEncrypted(w, cookie, ws.secret)
	if err != nil {
		log.Println(err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	ws.authUsers.AddUser(user, id)

	w.WriteHeader(http.StatusOK)
}

func (ws wrapperStruct) Login(w http.ResponseWriter, r *http.Request) {
	var body datamodels.Reg
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	password := utils.GetMD5Hash(body.Password)
	fmt.Println(password)
	id, ok := ws.DB.Login(body.Login, password)
	if ok != nil {
		status := mapErr(ok)
		http.Error(w, ok.Error(), status)
		return
	}
	user := utils.GenerateRandomString(3)
	cookie := http.Cookie{
		Name:     "UserID",
		Value:    user,
		Path:     "/api/user",
		HttpOnly: true,
		Secure:   false,
	}
	err = cookies.WriteEncrypted(w, cookie, ws.secret)
	if err != nil {
		log.Println(err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	ws.authUsers.AddUser(user, id)
	w.WriteHeader(http.StatusOK)
}

func (ws wrapperStruct) OrdersPost(w http.ResponseWriter, r *http.Request) {
	order, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, ok2 := ws.authUsers.GetUser(r.Context().Value(authenticatedUserKey).(string))
	if ok2 != nil {
		http.Error(w, "Unauthorized user", http.StatusUnauthorized)
		return
	}
	orderNum, err := strconv.Atoi(string(order))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ok := ws.DB.OrdersPost(datamodels.OrderInfo{UserID: id, OrderID: orderNum})
	if ok != nil {
		status := mapErr(ok)
		http.Error(w, ok.Error(), status)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
func (ws wrapperStruct) OrdersGet(w http.ResponseWriter, r *http.Request) {
	id, ok2 := ws.authUsers.GetUser(r.Context().Value(authenticatedUserKey).(string))
	if ok2 != nil {
		http.Error(w, "Unauthorized user", http.StatusUnauthorized)
		return
	}
	orderList, ok := ws.DB.GetOrderList(datamodels.OrderInfo{UserID: id})
	if ok != nil {
		status := mapErr(ok)
		http.Error(w, ok.Error(), status)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(orderList); err != nil {
		log.Println("jsonIndexPage: encoding response:", err)
		http.Error(w, "unable to encode response", http.StatusInternalServerError)
		return
	}
}
func (ws wrapperStruct) Balance(w http.ResponseWriter, r *http.Request) {
	id, ok2 := ws.authUsers.GetUser(r.Context().Value(authenticatedUserKey).(string))
	if ok2 != nil {
		http.Error(w, "Unauthorized user", http.StatusUnauthorized)
		return
	}
	balance, ok := ws.DB.Balance(datamodels.OrderInfo{UserID: id})
	if ok != nil {
		status := mapErr(ok)
		http.Error(w, ok.Error(), status)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(balance); err != nil {
		log.Println("jsonIndexPage: encoding response:", err)
		http.Error(w, "unable to encode response", http.StatusInternalServerError)
		return
	}
}
func (ws wrapperStruct) Withdraw(w http.ResponseWriter, r *http.Request) {

	var body datamodels.Withdraw
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, ok2 := ws.authUsers.GetUser(r.Context().Value(authenticatedUserKey).(string))
	if ok2 != nil {
		http.Error(w, "Unauthorized user", http.StatusUnauthorized)
		return
	}
	orderNum, _ := strconv.Atoi(body.Order)
	ok := ws.DB.Withdraw(datamodels.OrderInfo{UserID: id, OrderID: orderNum, Sum: body.Sum})
	if ok != nil {
		status := mapErr(ok)
		http.Error(w, ok.Error(), status)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (ws wrapperStruct) Withdrawals(w http.ResponseWriter, r *http.Request) {
	id, ok2 := ws.authUsers.GetUser(r.Context().Value(authenticatedUserKey).(string))
	if ok2 != nil {
		http.Error(w, "Unauthorized user", http.StatusUnauthorized)
		return
	}
	withdrawals, ok := ws.DB.GetWithdrawList(datamodels.OrderInfo{UserID: id})
	if ok != nil {
		status := mapErr(ok)
		http.Error(w, ok.Error(), status)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(withdrawals); err != nil {
		log.Println("jsonIndexPage: encoding response:", err)
		http.Error(w, "unable to encode response", http.StatusInternalServerError)
		return
	}
}

func mapErr(err error) int {
	if errors.Is(err, storage.ErrNotFound) {
		return http.StatusBadRequest
	}
	if errors.Is(err, storage.ErrInvalidOrder) {
		return http.StatusUnprocessableEntity
	}
	if errors.Is(err, storage.ErrWrongPassword) {
		return http.StatusUnauthorized
	}
	if errors.Is(err, storage.ErrAlreadyOrdered) {
		return http.StatusOK
	}
	if errors.Is(err, storage.ErrAnotherUserOrder) {
		return http.StatusConflict
	}
	if errors.Is(err, storage.ErrNoData) {
		return http.StatusNoContent
	}
	if errors.Is(err, storage.ErrNotEnoughMoney) {
		return http.StatusPaymentRequired
	}
	return http.StatusInternalServerError
}
