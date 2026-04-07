package server

import (
	"sync"
)

// User represents a user record in the database
type User struct {
	ID       int
	Name     string
	Password string
	Email    string
}

// MemoryDB is a simple in-memory database
type MemoryDB struct {
	mu     sync.RWMutex
	users  map[int]User
	nextID int
}

var DB *MemoryDB

// InitDB initializes the in-memory database with some default data
func InitDB() {
	DB = &MemoryDB{
		users:  make(map[int]User),
		nextID: 1,
	}

	DB.AddUser("labeo", "pass", "labeo@ea.com")
}

// AddUser adds a new user to the database
func (db *MemoryDB) AddUser(name, password, email string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	user := User{
		ID:       db.nextID,
		Name:     name,
		Password: password,
		Email:    email,
	}
	db.users[user.ID] = user
	db.nextID++
}

// GetUserByName retrieves a user by their name
func (db *MemoryDB) GetUserByName(name string) (User, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, user := range db.users {
		if user.Name == name {
			return user, true
		}
	}
	return User{}, false
}

// GetUserByID retrieves a user by their ID
func (db *MemoryDB) GetUserByID(id int) (User, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	user, ok := db.users[id]
	return user, ok
}
