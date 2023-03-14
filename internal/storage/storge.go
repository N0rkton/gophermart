package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/N0rkton/gophermart/internal/datamodels"
	"github.com/N0rkton/gophermart/internal/secondaryfunctions"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"log"
	"math"
	"strconv"

	"time"
)

// md5 cash for password
var (
	ErrNotFound         = errors.New("not found")
	ErrWrongPassword    = errors.New("invalid password")
	ErrInvalidOrder     = errors.New("invalid order number")
	ErrAlreadyOrdered   = errors.New("this order already exists")
	ErrAnotherUserOrder = errors.New("the order number has already been uploaded by another user")
	ErrInternal         = errors.New("server error")
	ErrNoData           = errors.New("no orders")
	ErrNotEnoughMoney   = errors.New("not enough money")
)

// иодель передаваемых данных
type Storage interface {
	Register(login string, password string) error
	Login(login string, password string) (int, error)
	OrdersPost(order datamodels.OrderInfo) error
	OrdersGet(order datamodels.OrderInfo) ([]datamodels.Order, error)
	Balance(order datamodels.OrderInfo) (datamodels.Balance, error)
	Withdraw(order datamodels.OrderInfo) error
	Withdrawals(order datamodels.OrderInfo) ([]datamodels.Withdrawals, error)
	GetAllOrdersForAccrual() ([]string, error)
	UpdateAccrual(accrual datamodels.Accrual) error
}
type DBStorage struct {
	db *sql.DB
}

func NewDBStorage(path string) (Storage, error) {
	if path == "" {
		return nil, errors.New("invalid db address")
	}
	db, err := sql.Open("pgx",
		path)
	if err != nil {
		return nil, err
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://./db/migrations",
		"postgres", driver)
	if err != nil {
		return nil, err
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}
	return &DBStorage{db: db}, nil
}
func (dbs *DBStorage) Register(login string, password string) error {
	_, err := dbs.db.Exec("insert into users (login, password) values ($1, $2);", login, password)
	return err
}
func (dbs *DBStorage) Login(login string, password string) (int, error) {
	rows := dbs.db.QueryRow("select id,password from users where login=$1 limit 1;", login)
	var v datamodels.Auth
	err := rows.Scan(&v.ID, &v.Password)
	if err != nil {
		return -1, ErrNotFound
	}
	if v.Password != password {
		return -1, ErrWrongPassword
	}
	return v.ID, nil
}
func (dbs *DBStorage) OrdersPost(order datamodels.OrderInfo) error {
	check := secondaryfunctions.Checksum(order.OrderID)
	fmt.Println(check)
	if check != 0 {
		return ErrInvalidOrder
	}
	orderTime := time.Now().UTC()
	_, err := dbs.db.Exec("insert into balance (user_id, order_id,created_at) values ($1, $2,$3);", order.UserID, strconv.Itoa(order.OrderID), orderTime.Format(time.RFC3339))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		rows := dbs.db.QueryRow("select user_id from balance where order_id=$1 limit 1;", strconv.Itoa(order.OrderID))
		var v int
		err := rows.Scan(&v)
		if err != nil {
			return ErrInternal
		}
		if v == order.UserID {
			return ErrAlreadyOrdered
		}
		return ErrAnotherUserOrder
	}
	if err != nil {
		return ErrInternal
	}
	return nil
}

func (dbs *DBStorage) OrdersGet(order datamodels.OrderInfo) ([]datamodels.Order, error) {

	rows, err := dbs.db.Query("select order_id,order_status,accrual, created_at from balance where user_id=$1 ORDER BY created_at DESC ;", order.UserID)
	if err != nil {
		return nil, ErrNoData
	}
	if rows.Err() != nil {
		return nil, ErrNoData
	}
	defer rows.Close()
	var resp []datamodels.Order
	var tmp datamodels.Order
	for rows.Next() {
		err = rows.Scan(&tmp.OrderID, &tmp.OrderStatus, &tmp.Accrual, &tmp.CreatedAt)
		if err != nil {
			return nil, ErrInternal
		}
		tmp.Accrual = tmp.Accrual / 100
		resp = append(resp, tmp)
	}
	return resp, nil
}

func (dbs *DBStorage) Balance(order datamodels.OrderInfo) (datamodels.Balance, error) {
	rows, err := dbs.db.Query("select accrual from balance where user_id=$1 and order_status='PROCESSED';", order.UserID)
	if err != nil {
		return datamodels.Balance{}, ErrNoData
	}
	if rows.Err() != nil {
		return datamodels.Balance{}, ErrNoData
	}
	defer rows.Close()
	var accrual float64
	var resp datamodels.Balance
	for rows.Next() {
		err = rows.Scan(&accrual)
		if err != nil {
			return datamodels.Balance{}, ErrInternal
		}
		resp.Current += accrual / 100
		if accrual < 0 {
			resp.Withdrawn += math.Abs(accrual / 100)
		}
	}

	return resp, nil
}
func (dbs *DBStorage) Withdraw(order datamodels.OrderInfo) error {
	check := secondaryfunctions.Checksum(order.OrderID)
	if check != 0 {
		return ErrInvalidOrder
	}
	userBalance, err := dbs.Balance(order)
	if err != nil {
		return ErrInternal
	}
	if userBalance.Current < order.Sum {
		return ErrNotEnoughMoney
	}
	orderTime := time.Now().Format(time.RFC3339)
	_, err = dbs.db.Exec("insert into balance (user_id, order_id,created_at,accrual,order_status) values ($1, $2,$3,$4,$5);", order.UserID, strconv.Itoa(order.OrderID), orderTime, int(-order.Sum*100), "PROCESSED")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return ErrInvalidOrder
	}
	if err != nil {
		return ErrInternal
	}
	return nil
}
func (dbs *DBStorage) Withdrawals(order datamodels.OrderInfo) ([]datamodels.Withdrawals, error) {

	rows, err := dbs.db.Query("select order_id,accrual, created_at from balance where user_id=$1 and accrual<0 ORDER BY created_at DESC ;", order.UserID)
	if err != nil {
		return nil, ErrNoData
	}
	if rows.Err() != nil {
		return nil, ErrNoData
	}
	defer rows.Close()
	var resp []datamodels.Withdrawals
	var tmp datamodels.Withdrawals
	for rows.Next() {
		err = rows.Scan(&tmp.Order, &tmp.Sum, &tmp.ProcessedAt)
		if err != nil {
			return nil, ErrInternal
		}
		tmp.Sum = math.Abs(tmp.Sum / 100)
		resp = append(resp, tmp)
	}
	return resp, nil
}
func (dbs *DBStorage) GetAllOrdersForAccrual() ([]string, error) {
	rows, err := dbs.db.Query("select order_id from balance where order_status!='INVALID' and order_status!='PROCESSED';")
	if err != nil {
		return nil, ErrNoData
	}
	if rows.Err() != nil {
		return nil, ErrNoData
	}
	var allOrders []string
	var tmp string
	for rows.Next() {
		err = rows.Scan(&tmp)
		if err != nil {
			return nil, ErrInternal
		}
		allOrders = append(allOrders, tmp)
	}
	return allOrders, nil
}
func (dbs *DBStorage) UpdateAccrual(accrual datamodels.Accrual) error {
	_, err := dbs.db.Exec("UPDATE balance SET accrual = $1, order_status=$2 WHERE order_id = $3 ;", accrual.Accrual, accrual.Status, accrual.Order)
	if err != nil {
		log.Panicln(err)
	}
	return nil
}
