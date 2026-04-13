package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Agent represents a call center agent
type Agent struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // never expose
	DisplayName  string `json:"displayName"`
	Extension    string `json:"extension"`
	SIPPassword  string `json:"sipPassword"`
	IsActive     bool   `json:"isActive"`
	CreatedAt    string `json:"createdAt"`
}

// Customer represents a CRM customer
type Customer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Company   string `json:"company"`
	Notes     string `json:"notes"`
	CallCount int    `json:"callCount"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CallLog represents a call record
type CallLog struct {
	ID           int64   `json:"id"`
	AgentID      int64   `json:"agentId"`
	CustomerID   *int64  `json:"customerId"`
	Extension    string  `json:"extension"`
	Direction    string  `json:"direction"` // "outbound" or "inbound"
	Status       string  `json:"status"`    // "answered", "no-answer", "busy", "failed"
	Duration     int     `json:"duration"`  // seconds
	StartedAt    string  `json:"startedAt"`
	EndedAt      *string `json:"endedAt"`
	CustomerName string  `json:"customerName,omitempty"`
}

// Task represents a follow-up task
type Task struct {
	ID           int64  `json:"id"`
	AgentID      int64  `json:"agentId"`
	CustomerID   *int64 `json:"customerId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	DueDate      string `json:"dueDate"`
	Status       string `json:"status"` // "pending", "done", "overdue"
	CustomerName string `json:"customerName,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

// SIPConfig represents the SIP server configuration
type SIPConfig struct {
	ID       int64  `json:"id"`
	Server   string `json:"server"` // host:port
	Domain   string `json:"domain"`
	IsActive bool   `json:"isActive"`
}

// initDB opens the MySQL database and creates tables
func initDB(mysqlDSN string) error {
	var err error
	db, err = sql.Open("mysql", mysqlDSN)
	if err != nil {
		return err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("MySQL connection failed: %w", err)
	}

	log.Printf("MySQL database initialized: %s", mysqlDSN)
	return nil
}

// --- Agent CRUD ---

func getAgentByUsername(username string) (*Agent, error) {
	a := &Agent{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, display_name, extension, sip_password, is_active, created_at FROM agents WHERE username = ?",
		username,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &a.DisplayName, &a.Extension, &a.SIPPassword, &a.IsActive, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func getAgentByID(id int64) (*Agent, error) {
	a := &Agent{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, display_name, extension, sip_password, is_active, created_at FROM agents WHERE id = ?",
		id,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &a.DisplayName, &a.Extension, &a.SIPPassword, &a.IsActive, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func listAgents() ([]Agent, error) {
	rows, err := db.Query("SELECT id, username, password_hash, display_name, extension, sip_password, is_active, created_at FROM agents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.DisplayName, &a.Extension, &a.SIPPassword, &a.IsActive, &a.CreatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

// --- Customer CRUD ---

func listCustomers() ([]Customer, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.phone, c.email, c.company, c.notes, c.created_at, c.updated_at,
			COALESCE((SELECT COUNT(*) FROM calls WHERE customer_id = c.id), 0) as call_count
		FROM customers c ORDER BY c.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var customers []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Company, &c.Notes, &c.CreatedAt, &c.UpdatedAt, &c.CallCount); err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, nil
}

func getCustomerByID(id int64) (*Customer, error) {
	c := &Customer{}
	err := db.QueryRow(`
		SELECT id, name, phone, email, company, notes, created_at, updated_at,
			COALESCE((SELECT COUNT(*) FROM calls WHERE customer_id = c.id), 0)
		FROM customers c WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Company, &c.Notes, &c.CreatedAt, &c.UpdatedAt, &c.CallCount)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func createCustomer(c *Customer) error {
	res, err := db.Exec(
		"INSERT INTO customers (name, phone, email, company, notes) VALUES (?, ?, ?, ?, ?)",
		c.Name, c.Phone, c.Email, c.Company, c.Notes,
	)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	c.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	c.UpdatedAt = c.CreatedAt
	return nil
}

func updateCustomer(c *Customer) error {
	_, err := db.Exec(
		"UPDATE customers SET name=?, phone=?, email=?, company=?, notes=?, updated_at=NOW() WHERE id=?",
		c.Name, c.Phone, c.Email, c.Company, c.Notes, c.ID,
	)
	return err
}

func deleteCustomer(id int64) error {
	_, err := db.Exec("DELETE FROM customers WHERE id=?", id)
	return err
}

// --- CallLog CRUD ---

func createCallLog(agentID int64, customerID *int64, extension, direction string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO calls (agent_id, customer_id, extension, direction, status, started_at) VALUES (?, ?, ?, ?, 'ringing', NOW())",
		agentID, customerID, extension, direction,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateCallStatus(callID int64, status string, duration int) error {
	_, err := db.Exec(
		"UPDATE calls SET status=?, duration=?, ended_at=NOW() WHERE id=?",
		status, duration, callID,
	)
	return err
}

func listCallsByAgent(agentID int64, limit int) ([]CallLog, error) {
	rows, err := db.Query(`
		SELECT c.id, c.agent_id, c.customer_id, c.extension, c.direction, c.status, c.duration, c.started_at, c.ended_at,
			COALESCE(cust.name, '')
		FROM calls c
		LEFT JOIN customers cust ON c.customer_id = cust.id
		WHERE c.agent_id = ?
		ORDER BY c.started_at DESC LIMIT ?`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []CallLog
	for rows.Next() {
		var cl CallLog
		if err := rows.Scan(&cl.ID, &cl.AgentID, &cl.CustomerID, &cl.Extension, &cl.Direction, &cl.Status, &cl.Duration, &cl.StartedAt, &cl.EndedAt, &cl.CustomerName); err != nil {
			return nil, err
		}
		calls = append(calls, cl)
	}
	return calls, nil
}

func listRecentCalls(limit int) ([]CallLog, error) {
	rows, err := db.Query(`
		SELECT c.id, c.agent_id, c.customer_id, c.extension, c.direction, c.status, c.duration, c.started_at, c.ended_at,
			COALESCE(cust.name, '')
		FROM calls c
		LEFT JOIN customers cust ON c.customer_id = cust.id
		ORDER BY c.started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []CallLog
	for rows.Next() {
		var cl CallLog
		if err := rows.Scan(&cl.ID, &cl.AgentID, &cl.CustomerID, &cl.Extension, &cl.Direction, &cl.Status, &cl.Duration, &cl.StartedAt, &cl.EndedAt, &cl.CustomerName); err != nil {
			return nil, err
		}
		calls = append(calls, cl)
	}
	return calls, nil
}

// --- Task CRUD ---

func listTasksByAgent(agentID int64, statusFilter string) ([]Task, error) {
	query := `
		SELECT t.id, t.agent_id, t.customer_id, t.title, t.description, t.due_date, t.status, t.created_at,
			COALESCE(cust.name, '')
		FROM tasks t
		LEFT JOIN customers cust ON t.customer_id = cust.id
		WHERE t.agent_id = ?`
	args := []interface{}{agentID}
	if statusFilter != "" {
		query += " AND t.status = ?"
		args = append(args, statusFilter)
	}
	query += " ORDER BY t.due_date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.AgentID, &t.CustomerID, &t.Title, &t.Description, &t.DueDate, &t.Status, &t.CreatedAt, &t.CustomerName); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func createTask(t *Task) error {
	res, err := db.Exec(
		"INSERT INTO tasks (agent_id, customer_id, title, description, due_date, status) VALUES (?, ?, ?, ?, ?, ?)",
		t.AgentID, t.CustomerID, t.Title, t.Description, t.DueDate, t.Status,
	)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func updateTask(t *Task) error {
	_, err := db.Exec(
		"UPDATE tasks SET title=?, description=?, due_date=?, status=?, customer_id=? WHERE id=? AND agent_id=?",
		t.Title, t.Description, t.DueDate, t.Status, t.CustomerID, t.ID, t.AgentID,
	)
	return err
}

func deleteTask(id int64, agentID int64) error {
	_, err := db.Exec("DELETE FROM tasks WHERE id=? AND agent_id=?", id, agentID)
	return err
}

// --- SIPConfig ---

func getActiveSIPConfig() (*SIPConfig, error) {
	sc := &SIPConfig{}
	err := db.QueryRow("SELECT id, server, domain, is_active FROM sip_config WHERE is_active = 1 LIMIT 1").Scan(&sc.ID, &sc.Server, &sc.Domain, &sc.IsActive)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func updateSIPConfig(sc *SIPConfig) error {
	_, err := db.Exec("UPDATE sip_config SET server=?, domain=?, is_active=? WHERE id=?", sc.Server, sc.Domain, sc.IsActive, sc.ID)
	return err
}
