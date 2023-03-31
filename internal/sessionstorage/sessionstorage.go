package sessionstorage

import (
	"errors"
	"sync"
)

type SessionStorage interface {
	AddUser(user string, id int) error
	GetUser(user string) (int, error)
}
type authUsersStorage struct {
	authUsers map[string]int
	mutex     sync.RWMutex
}

func NewAuthUsersStorage() SessionStorage {
	var mutex sync.RWMutex
	return &authUsersStorage{authUsers: make(map[string]int), mutex: mutex}
}
func (us *authUsersStorage) AddUser(user string, id int) error {
	us.mutex.Lock()
	us.authUsers[user] = id
	us.mutex.Unlock()
	return nil
}
func (us *authUsersStorage) GetUser(user string) (int, error) {
	us.mutex.RLock()
	id, ok := us.authUsers[user]
	if !ok {
		return 0, errors.New("user not found")
	}
	us.mutex.RUnlock()
	return id, nil
}
