package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Created  string `json:"created"`
}

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type PollingJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type PollingJobRequest struct {
	MinSleepTime int `json:"minSleepTime"`
}

type JobOperation struct {
	Type   string
	JobID  string
	Job    *PollingJob
	Result chan *PollingJob
}

var (
	users      = make(map[int]*User)
	products   = make(map[int]*Product)
	usersMux   = sync.RWMutex{}
	prodMux    = sync.RWMutex{}
	nextUserID = 1
	nextProdID = 1
	jobChannel = make(chan JobOperation)
)

func init() {
	// Seed some initial data
	products[1] = &Product{ID: 1, Name: "Laptop", Description: "High-performance laptop", Price: 999.99, Stock: 50}
	products[2] = &Product{ID: 2, Name: "Mouse", Description: "Wireless optical mouse", Price: 29.99, Stock: 100}
	products[3] = &Product{ID: 3, Name: "Keyboard", Description: "Mechanical keyboard", Price: 79.99, Stock: 75}
	nextProdID = 4
}

func main() {
	// Start job manager goroutine
	go jobManager()

	http.HandleFunc("/health", healthHandler)

	// User endpoints
	http.HandleFunc("/api/users", usersHandler)
	http.HandleFunc("/api/users/", userHandler)

	// Product endpoints
	http.HandleFunc("/api/products", productsHandler)
	http.HandleFunc("/api/products/", productHandler)

	// Special endpoints for testing
	http.HandleFunc("/api/slow", slowHandler)
	http.HandleFunc("/api/random-error", randomErrorHandler)
	http.HandleFunc("/api/echo", echoHandler)

	// Polling job endpoints
	http.HandleFunc("/api/pollingjob", pollingJobHandler)
	http.HandleFunc("/api/pollingjob/", pollingJobStatusHandler)

	fmt.Println("Test API Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	respondJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "API is healthy",
		Data:    map[string]string{"timestamp": time.Now().Format(time.RFC3339)},
	})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getUsersHandler(w, r)
	case "POST":
		createUserHandler(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path /api/users/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	if path == "" {
		respondError(w, http.StatusBadRequest, "User ID required")
		return
	}

	id, err := strconv.Atoi(path)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	switch r.Method {
	case "GET":
		getUserHandler(w, r, id)
	case "PUT":
		updateUserHandler(w, r, id)
	case "DELETE":
		deleteUserHandler(w, r, id)
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func productsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getProductsHandler(w, r)
	case "POST":
		createProductHandler(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func productHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path /api/products/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/products/")
	if path == "" {
		respondError(w, http.StatusBadRequest, "Product ID required")
		return
	}

	id, err := strconv.Atoi(path)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	if r.Method == "GET" {
		getProductHandler(w, r, id)
	} else {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usersMux.Lock()
	user.ID = nextUserID
	nextUserID++
	user.Created = time.Now().Format(time.RFC3339)
	users[user.ID] = &user
	usersMux.Unlock()

	respondJSON(w, http.StatusCreated, APIResponse{
		Status: "success",
		Data:   map[string]*User{"user": &user},
	})
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	usersMux.RLock()
	userList := make([]*User, 0, len(users))
	for _, user := range users {
		userList = append(userList, user)
	}
	usersMux.RUnlock()

	respondJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   map[string][]*User{"users": userList},
	})
}

func getUserHandler(w http.ResponseWriter, r *http.Request, id int) {
	usersMux.RLock()
	user, exists := users[id]
	usersMux.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	respondJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   user,
	})
}

func updateUserHandler(w http.ResponseWriter, r *http.Request, id int) {
	var updatedUser User
	if err := json.NewDecoder(r.Body).Decode(&updatedUser); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usersMux.Lock()
	user, exists := users[id]
	if !exists {
		usersMux.Unlock()
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	user.Username = updatedUser.Username
	user.Email = updatedUser.Email
	usersMux.Unlock()

	respondJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   user,
	})
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request, id int) {
	usersMux.Lock()
	_, exists := users[id]
	if !exists {
		usersMux.Unlock()
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	delete(users, id)
	usersMux.Unlock()

	respondJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "User deleted successfully",
	})
}

func getProductsHandler(w http.ResponseWriter, r *http.Request) {
	prodMux.RLock()
	productList := make([]*Product, 0, len(products))
	for _, product := range products {
		productList = append(productList, product)
	}
	prodMux.RUnlock()

	respondJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   map[string][]*Product{"products": productList},
	})
}

func getProductHandler(w http.ResponseWriter, r *http.Request, id int) {
	prodMux.RLock()
	product, exists := products[id]
	prodMux.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "Product not found")
		return
	}

	respondJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   product,
	})
}

func createProductHandler(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	prodMux.Lock()
	product.ID = nextProdID
	nextProdID++
	products[product.ID] = &product
	prodMux.Unlock()

	respondJSON(w, http.StatusCreated, APIResponse{
		Status: "success",
		Data:   map[string]*Product{"product": &product},
	})
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Simulate slow response (1-3 seconds)
	delay := time.Duration(rand.Intn(2000)+1000) * time.Millisecond
	time.Sleep(delay)

	respondJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: fmt.Sprintf("Slow response completed after %v", delay),
		Data:    map[string]string{"delay": delay.String()},
	})
}

func randomErrorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 30% chance of error
	if rand.Intn(100) < 30 {
		respondError(w, http.StatusInternalServerError, "Random server error occurred")
		return
	}

	respondJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "Request succeeded",
	})
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	respondJSON(w, http.StatusOK, APIResponse{
		Status:  "success",
		Message: "Echo successful",
		Data:    payload,
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, APIResponse{
		Status:  "error",
		Message: message,
	})
}

func jobManager() {
	jobs := make(map[string]*PollingJob)

	for op := range jobChannel {
		log.Default().Printf("job channel received %v", op)
		switch op.Type {
		case "create":
			jobs[op.Job.ID] = op.Job
			op.Result <- op.Job
		case "get":
			if job, exists := jobs[op.JobID]; exists {
				op.Result <- job
			} else {
				op.Result <- nil
			}
		case "update":
			if job, exists := jobs[op.JobID]; exists {
				job.Status = op.Job.Status
			}
			close(op.Result)
		}
	}
}

func pollingJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse JSON body to get minSleepTime
	var requestBody PollingJobRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		// If no valid JSON body, use default minSleepTime of 500ms
		requestBody.MinSleepTime = 500
	}

	// Validate minSleepTime (minimum 100ms, default 500ms if 0 or negative)
	minSleepTime := requestBody.MinSleepTime
	if minSleepTime <= 0 {
		minSleepTime = 500
	}
	if minSleepTime < 100 {
		minSleepTime = 100
	}

	// Generate unique job ID
	jobID := fmt.Sprintf("job_%d_%d", time.Now().Unix(), rand.Intn(10000))

	// Create job with pending status
	job := &PollingJob{
		ID:     jobID,
		Status: "pending",
	}

	// Store job via channel
	resultChan := make(chan *PollingJob)
	jobChannel <- JobOperation{
		Type:   "create",
		Job:    job,
		Result: resultChan,
	}
	<-resultChan

	// Start background goroutine
	go func() {
		// Use configurable delay with some randomness (minSleepTime + 0-500ms)
		delay := time.Duration(minSleepTime+rand.Intn(500)) * time.Millisecond
		time.Sleep(delay)

		// Update job status to completed via channel
		updateResultChan := make(chan *PollingJob)
		jobChannel <- JobOperation{
			Type:   "update",
			JobID:  jobID,
			Job:    &PollingJob{Status: "completed"},
			Result: updateResultChan,
		}
		<-updateResultChan
	}()

	respondJSON(w, http.StatusAccepted, APIResponse{
		Status: "success",
		Data:   job,
	})
}

func pollingJobStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract ID from path /api/pollingjob/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/pollingjob/")
	if path == "" {
		respondError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	// Get job via channel
	resultChan := make(chan *PollingJob)
	jobChannel <- JobOperation{
		Type:   "get",
		JobID:  path,
		Result: resultChan,
	}
	job := <-resultChan

	if job == nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, APIResponse{
		Status: "success",
		Data:   job,
	})
}
