package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/awlx/bmwtools/pkg/data"
)

// SessionData represents the data associated with a user session
type SessionData struct {
	ID           string
	DataManager  *data.Manager
	CreatedAt    time.Time
	LastAccessed time.Time
}

// SessionStore manages user sessions and their associated data
type SessionStore struct {
	sessions map[string]*SessionData
	mutex    sync.RWMutex
	// Time after which inactive sessions are cleaned up (e.g., 30 minutes)
	expirationTime time.Duration
}

// NewSessionStore creates a new session store
func NewSessionStore(expirationTime time.Duration) *SessionStore {
	store := &SessionStore{
		sessions:       make(map[string]*SessionData),
		expirationTime: expirationTime,
	}

	// Start a goroutine to periodically clean up expired sessions
	go store.cleanupExpiredSessions()

	return store
}

// cleanupExpiredSessions periodically removes expired sessions
func (s *SessionStore) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()

		now := time.Now()
		for id, session := range s.sessions {
			if now.Sub(session.LastAccessed) > s.expirationTime {
				delete(s.sessions, id)
			}
		}

		s.mutex.Unlock()
	}
}

// CreateSession creates a new session and returns its ID
func (s *SessionStore) CreateSession() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Generate a random ID using crypto/rand
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to current time if random generation fails
		sessionID := hex.EncodeToString([]byte(time.Now().String()))
		now := time.Now()

		s.sessions[sessionID] = &SessionData{
			ID:           sessionID,
			DataManager:  data.NewManager(),
			CreatedAt:    now,
			LastAccessed: now,
		}

		return sessionID
	}

	sessionID := hex.EncodeToString(bytes)
	now := time.Now()

	s.sessions[sessionID] = &SessionData{
		ID:           sessionID,
		DataManager:  data.NewManager(),
		CreatedAt:    now,
		LastAccessed: now,
	}

	return sessionID
}

// GetSession returns the session data for the given ID
func (s *SessionStore) GetSession(sessionID string) (*SessionData, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if exists {
		// Update last accessed time
		session.LastAccessed = time.Now()
	}

	return session, exists
}

// GetOrCreateSession gets an existing session or creates a new one
func (s *SessionStore) GetOrCreateSession(sessionID string) (string, *SessionData) {
	// If we have a session ID and it exists, return it
	if sessionID != "" {
		if session, exists := s.GetSession(sessionID); exists {
			return sessionID, session
		}
	}

	// Create a new session
	newID := s.CreateSession()
	session, _ := s.GetSession(newID)
	return newID, session
}
