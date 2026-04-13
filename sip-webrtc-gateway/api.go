package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// JWT-like session token (simplified HMAC-SHA256)
var jwtSecret = []byte("sip-crm-secret-change-me")

// session stores logged-in agent info
type session struct {
	AgentID  int64  `json:"agentId"`
	Username string `json:"username"`
}

// authMiddleware validates the session token from Authorization header
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}
		token = strings.TrimPrefix(token, "Bearer ")

		sess, err := parseToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		// Store session in request context
		r.Header.Set("X-Agent-ID", fmt.Sprintf("%d", sess.AgentID))
		r.Header.Set("X-Username", sess.Username)
		next(w, r)
	}
}

func generateToken(agentID int64, username string) (string, error) {
	payload := fmt.Sprintf("%d:%s:%d", agentID, username, time.Now().Add(24*time.Hour).Unix())
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig), nil
}

func parseToken(token string) (*session, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(parts[0]))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Parse payload: "agentID:username:expiry"
	payloadParts := strings.SplitN(parts[0], ":", 3)
	if len(payloadParts) != 3 {
		return nil, fmt.Errorf("invalid payload")
	}

	expiry, err := strconv.ParseInt(payloadParts[2], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return nil, fmt.Errorf("token expired")
	}

	agentID, err := strconv.ParseInt(payloadParts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID")
	}

	return &session{AgentID: agentID, Username: payloadParts[1]}, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Auth handlers ---

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	agent, err := getAgentByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Simple password check (replace with bcrypt in production)
	if agent.PasswordHash != req.Password {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !agent.IsActive {
		writeError(w, http.StatusForbidden, "account disabled")
		return
	}

	token, err := generateToken(agent.ID, agent.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":       token,
		"agentId":     agent.ID,
		"username":    agent.Username,
		"displayName": agent.DisplayName,
		"extension":   agent.Extension,
	})
}

func handleGetProfile(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)
	agent, err := getAgentByID(agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":          agent.ID,
		"username":    agent.Username,
		"displayName": agent.DisplayName,
		"extension":   agent.Extension,
		"isActive":    agent.IsActive,
	})
}

// --- Customer handlers ---

func handleListCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := listCustomers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if customers == nil {
		customers = []Customer{}
	}
	writeJSON(w, http.StatusOK, customers)
}

func handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := getCustomerByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "customer not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var c Customer
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if c.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := createCustomer(&c); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func handleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	var c Customer
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if c.ID == 0 {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := updateCustomer(&c); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func handleDeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := deleteCustomer(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Call handlers ---

func handleListCalls(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	calls, err := listCallsByAgent(agentID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if calls == nil {
		calls = []CallLog{}
	}
	writeJSON(w, http.StatusOK, calls)
}

func handleListRecentCalls(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	calls, err := listRecentCalls(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if calls == nil {
		calls = []CallLog{}
	}
	writeJSON(w, http.StatusOK, calls)
}

// --- Task handlers ---

func handleListTasks(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)
	statusFilter := r.URL.Query().Get("status")
	tasks, err := listTasksByAgent(agentID, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func handleCreateTask(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)
	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	t.AgentID = agentID
	if t.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if t.Status == "" {
		t.Status = "pending"
	}
	if err := createTask(&t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)
	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	t.AgentID = agentID
	if err := updateTask(&t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := deleteTask(id, agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- SIP Config handler ---

func handleGetSIPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := getActiveSIPConfig()
	if err != nil {
		writeError(w, http.StatusNotFound, "no active SIP config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func handleUpdateSIPConfig(w http.ResponseWriter, r *http.Request) {
	var cfg SIPConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := updateSIPConfig(&cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("SIP config updated: server=%s domain=%s", cfg.Server, cfg.Domain)
	writeJSON(w, http.StatusOK, cfg)
}

// --- Dashboard stats ---

func handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	agentID, _ := strconv.ParseInt(r.Header.Get("X-Agent-ID"), 10, 64)

	var totalCalls, answeredCalls, pendingTasks int
	db.QueryRow("SELECT COUNT(*) FROM calls WHERE agent_id = ?", agentID).Scan(&totalCalls)
	db.QueryRow("SELECT COUNT(*) FROM calls WHERE agent_id = ? AND status = 'answered'", agentID).Scan(&answeredCalls)
	db.QueryRow("SELECT COUNT(*) FROM tasks WHERE agent_id = ? AND status = 'pending'", agentID).Scan(&pendingTasks)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"totalCalls":    totalCalls,
		"answeredCalls": answeredCalls,
		"pendingTasks":  pendingTasks,
	})
}

// registerAPIRoutes sets up all REST API routes
func registerAPIRoutes(mux *http.ServeMux) {
	// Auth (no token required)
	mux.HandleFunc("/api/login", handleLogin)

	// Profile
	mux.HandleFunc("/api/profile", authMiddleware(handleGetProfile))

	// Customers
	mux.HandleFunc("/api/customers", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleListCustomers(w, r)
		case http.MethodPost:
			handleCreateCustomer(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/customers/detail", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetCustomer(w, r)
		case http.MethodPut:
			handleUpdateCustomer(w, r)
		case http.MethodDelete:
			handleDeleteCustomer(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Calls
	mux.HandleFunc("/api/calls", authMiddleware(handleListCalls))
	mux.HandleFunc("/api/calls/recent", authMiddleware(handleListRecentCalls))

	// Tasks
	mux.HandleFunc("/api/tasks", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleListTasks(w, r)
		case http.MethodPost:
			handleCreateTask(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/tasks/detail", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			handleUpdateTask(w, r)
		case http.MethodDelete:
			handleDeleteTask(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// SIP Config
	mux.HandleFunc("/api/sip-config", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetSIPConfig(w, r)
		case http.MethodPut:
			handleUpdateSIPConfig(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Dashboard
	mux.HandleFunc("/api/dashboard", authMiddleware(handleDashboardStats))

	log.Println("API routes registered")
}
