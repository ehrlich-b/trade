package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"trade/internal/store"
)

// Session represents an authenticated session
type Session struct {
	Token     string
	UserID    string
	AccountID string
	ExpiresAt time.Time
}

// SessionStore manages active sessions with database persistence
type SessionStore struct {
	store  *store.Store
	mu     sync.RWMutex
	cache  map[string]*Session // In-memory cache for performance
	stopCh chan struct{}
}

func NewSessionStore(s *store.Store) *SessionStore {
	ss := &SessionStore{
		store:  s,
		cache:  make(map[string]*Session),
		stopCh: make(chan struct{}),
	}
	go ss.cleanupLoop()
	return ss
}

// cleanupLoop periodically removes expired sessions
func (ss *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ss.cleanup()
		case <-ss.stopCh:
			return
		}
	}
}

// cleanup removes all expired sessions
func (ss *SessionStore) cleanup() {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Clean in-memory cache
	now := time.Now()
	for token, session := range ss.cache {
		if now.After(session.ExpiresAt) {
			delete(ss.cache, token)
		}
	}
	// Clean database
	if ss.store != nil {
		ss.store.CleanupExpiredSessions()
	}
}

// Stop halts the cleanup goroutine
func (ss *SessionStore) Stop() {
	close(ss.stopCh)
}

func (ss *SessionStore) Create(userID, accountID string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	token := generateToken()
	expiresAt := time.Now().Add(24 * time.Hour)
	session := &Session{
		Token:     token,
		UserID:    userID,
		AccountID: accountID,
		ExpiresAt: expiresAt,
	}

	// Save to database
	if ss.store != nil {
		ss.store.CreateSession(token, userID, accountID, expiresAt)
	}

	// Cache in memory
	ss.cache[token] = session
	return session
}

func (ss *SessionStore) Get(token string) *Session {
	ss.mu.RLock()
	// Check cache first
	if session, ok := ss.cache[token]; ok {
		if time.Now().Before(session.ExpiresAt) {
			ss.mu.RUnlock()
			return session
		}
	}
	ss.mu.RUnlock()

	// Not in cache or expired, check database
	if ss.store != nil {
		dbSession, err := ss.store.GetSession(token)
		if err == nil && dbSession != nil {
			session := &Session{
				Token:     dbSession.Token,
				UserID:    dbSession.UserID,
				AccountID: dbSession.AccountID,
				ExpiresAt: dbSession.ExpiresAt,
			}
			// Update cache
			ss.mu.Lock()
			ss.cache[token] = session
			ss.mu.Unlock()
			return session
		}
	}

	return nil
}

func (ss *SessionStore) Delete(token string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.cache, token)
	if ss.store != nil {
		ss.store.DeleteSession(token)
	}
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	AccountID string `json:"account_id"`
	Username  string `json:"username"`
}

type AccountResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Balance   int64  `json:"balance"`
	Positions []PositionResponse `json:"positions"`
}

type PositionResponse struct {
	Symbol      string `json:"symbol"`
	Quantity    int64  `json:"quantity"`
	AvgPrice    int64  `json:"avg_price"`
	RealizedPnL int64  `json:"realized_pnl"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	if len(req.Username) < 3 || len(req.Username) > 32 {
		http.Error(w, "username must be 3-32 characters", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		http.Error(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	user, err := s.store.CreateUser(req.Username, req.Password)
	if err == store.ErrUserExists {
		http.Error(w, "username already taken", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	account, err := s.store.GetAccountByUserID(user.ID)
	if err != nil {
		http.Error(w, "failed to get account", http.StatusInternalServerError)
		return
	}

	session := s.sessions.Create(user.ID, account.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token:     session.Token,
		UserID:    user.ID,
		AccountID: account.ID,
		Username:  user.Username,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := s.store.AuthenticateUser(req.Username, req.Password)
	if err == store.ErrUserNotFound || err == store.ErrInvalidPassword {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	account, err := s.store.GetAccountByUserID(user.ID)
	if err != nil {
		http.Error(w, "failed to get account", http.StatusInternalServerError)
		return
	}

	session := s.sessions.Create(user.ID, account.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token:     session.Token,
		UserID:    user.ID,
		AccountID: account.ID,
		Username:  user.Username,
	})
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	account, err := s.store.GetAccountByID(session.AccountID)
	if err != nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	user, err := s.store.GetUserByID(session.UserID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	positions, err := s.store.GetAllPositions(session.AccountID)
	if err != nil {
		http.Error(w, "failed to get positions", http.StatusInternalServerError)
		return
	}

	posResponses := make([]PositionResponse, 0, len(positions))
	for _, pos := range positions {
		posResponses = append(posResponses, PositionResponse{
			Symbol:      pos.Symbol,
			Quantity:    pos.Quantity,
			AvgPrice:    pos.AvgPrice,
			RealizedPnL: pos.RealizedPnL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AccountResponse{
		ID:        account.ID,
		UserID:    user.ID,
		Username:  user.Username,
		Balance:   account.Balance,
		Positions: posResponses,
	})
}

func (s *Server) getSession(r *http.Request) *Session {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil
	}

	return s.sessions.Get(parts[1])
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.GetLeaderboard(10)
	if err != nil {
		http.Error(w, "failed to get leaderboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
