package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"math"

	"time"
)

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

type Storage interface {
	Register(login string, password string) error
	Login(login string, password string) (int, error)
	OrdersPost(id int, order int) error
	OrdersGet(id int) ([]byte, error)
	Balance(id int) ([]byte, error)
	Withdraw(id int, order int, sum float32) error
	Withdrawals(id int) ([]byte, error)
}
type DBStorage struct {
	DB *sql.DB
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
	return &DBStorage{DB: db}, nil
}
func (dbs *DBStorage) Register(login string, password string) error {
	_, err := dbs.DB.Exec("insert into users (login, password) values ($1, $2);", login, password)
	return err
}
func (dbs *DBStorage) Login(login string, password string) (int, error) {
	rows := dbs.DB.QueryRow("select id,password from users where login=$1 limit 1;", login)
	type auth struct {
		id       int
		password string
	}
	var v auth
	err := rows.Scan(&v.id, &v.password)
	if err != nil {
		return -1, ErrNotFound
	}
	if v.password != password {
		return -1, ErrWrongPassword
	}
	return v.id, nil
}
func (dbs *DBStorage) OrdersPost(id int, order int) error {
	check := calculateLuhn(order)
	if check != order%10 {
		return ErrInvalidOrder
	}
	orderTime := time.Now().Format(time.RFC3339)
	_, err := dbs.DB.Exec("insert into balance (user_id, order_id,created_at) values ($1, $2,$3);", id, order, orderTime)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		rows := dbs.DB.QueryRow("select user_id from balance where order_id=$1 limit 1;", order)
		var v int
		err := rows.Scan(&v)
		if err != nil {
			return ErrInternal
		}
		if v == id {
			return ErrAlreadyOrdered
		}
		return ErrAnotherUserOrder
	}
	return nil
}
func calculateLuhn(number int) int {
	checkNumber := checksum(number)
	if checkNumber == 0 {
		return 0
	}
	return 10 - checkNumber
}
func checksum(number int) int {
	var luhn int
	for i := 0; number > 0; i++ {
		cur := number % 10
		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}
		luhn += cur
		number = number / 10
	}
	return luhn % 10
}

func (dbs *DBStorage) OrdersGet(id int) ([]byte, error) {
	type balance struct {
		Order_id     int       `json:"order_id"`
		Order_status string    `json:"order_status"`
		Accrual      float32   `json:"accrual"`
		Created_at   time.Time `json:"created_at"`
	}
	rows, err := dbs.DB.Query("select order_id,order_status,accrual, created_at from balance where user_id=$1 ORDER BY created_at DESC ;", id)
	if err != nil {
		return nil, ErrNoData
	}
	var resp []balance
	var tmp balance
	for rows.Next() {
		err = rows.Scan(&tmp.Order_id, &tmp.Order_status, &tmp.Accrual, &tmp.Created_at)
		if err != nil {
			return nil, ErrInternal
		}
		resp = append(resp, tmp)
	}
	var w []byte
	w, err = json.Marshal(resp)
	if err != nil {
		return nil, ErrInternal
	}
	return w, nil
}

type balance struct {
	Current   float32 `json:"current"`
	Withdrawn float32 `json:"withdrawn"`
}

func (dbs *DBStorage) Balance(id int) ([]byte, error) {
	rows, err := dbs.DB.Query("select accrual from balance where user_id=$1 and order_status='PROCESSED';", id)
	if err != nil {
		return nil, ErrNoData
	}
	var accrual float32
	var resp balance
	for rows.Next() {
		err = rows.Scan(&accrual)
		if err != nil {
			return nil, ErrInternal
		}
		resp.Current += accrual
		if accrual < 0 {
			resp.Withdrawn += float32(math.Abs(float64(accrual)))
		}
	}
	var w []byte
	w, err = json.Marshal(resp)
	if err != nil {
		return nil, ErrInternal
	}
	return w, nil
}
func (dbs *DBStorage) Withdraw(id int, order int, sum float32) error {
	check := calculateLuhn(order)
	if check != order%10 {
		return ErrInvalidOrder
	}
	bal, err := dbs.Balance(id)
	if err != nil {
		return ErrInternal
	}
	var userBalance balance
	err = json.Unmarshal(bal, &userBalance)
	if err != nil {
		return ErrInternal
	}
	if userBalance.Current < sum {
		return ErrNotEnoughMoney
	}
	orderTime := time.Now().Format(time.RFC3339)
	_, err = dbs.DB.Exec("insert into balance (user_id, order_id,created_at,accrual,order_status) values ($1, $2,$3,$4,$5);", id, order, orderTime, -sum, "PROCESSED")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return ErrInvalidOrder
	}
	return nil
}
func (dbs *DBStorage) Withdrawals(id int) ([]byte, error) {
	type withdrawals struct {
		Order        int       `json:"order"`
		Sum          float32   `json:"sum"`
		Processed_at time.Time `json:"processed_at"`
	}
	rows, err := dbs.DB.Query("select order_id,accrual, created_at from balance where user_id=$1 and accrual<0 ORDER BY created_at DESC ;", id)
	if err != nil {
		return nil, ErrNoData
	}
	var resp []withdrawals
	var tmp withdrawals
	for rows.Next() {
		err = rows.Scan(&tmp.Order, &tmp.Sum, &tmp.Processed_at)
		if err != nil {
			return nil, ErrInternal
		}
		tmp.Sum = float32(math.Abs(float64(tmp.Sum)))
		resp = append(resp, tmp)
	}
	var w []byte
	w, err = json.Marshal(resp)
	if err != nil {
		return nil, ErrInternal
	}
	return w, nil
}
