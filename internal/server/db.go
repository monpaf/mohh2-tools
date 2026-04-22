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
type memoryDB struct {
	mu     sync.RWMutex
	users  map[int]*User
	nextID int
}

// newDB initializes the in-memory database with some default data
func newDB() *memoryDB {
	db := &memoryDB{
		users:  make(map[int]*User),
		nextID: 1,
	}

	db.addUser("labeo", "pass", "labeo@ea.com")
	db.addUser("gigi", "pass", "labeo@ea.com")

	return db
}

// addUser adds a new user to the database
func (db *memoryDB) addUser(name, password, email string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	id := db.nextID
	db.users[id] = &User{
		ID:       db.nextID,
		Name:     name,
		Password: password,
		Email:    email,
	}
	db.nextID++
}

// GetUserByName retrieves a user by their name
func (db *memoryDB) GetUserByName(name string) (*User, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, user := range db.users {
		if user.Name == name {
			return user, true
		}
	}
	return nil, false
}

// GetUserByID retrieves a user by their ID
func (db *memoryDB) GetUserByID(id int) (*User, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	user, exists := db.users[id]
	return user, exists
}
