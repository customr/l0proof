package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb/util"
)

type RPCServer struct {
	operator *OperatorNode
	port     string
	server   *http.Server
}

func NewRPCServer(operator *OperatorNode, port string) *RPCServer {
	return &RPCServer{
		operator: operator,
		port:     port,
	}
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func timeoutMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		r = r.WithContext(ctx)

		done := make(chan bool, 1)
		go func() {
			h(w, r)
			done <- true
		}()

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write([]byte("Request timed out"))
			}
		case <-done:
			// Request completed before timeout
		}
	}
}

func logMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h(w, r)
		log.Printf("API Request: %s %s (took: %v)", r.Method, r.URL.Path, time.Since(start))
	}
}

func (s *RPCServer) wrapHandler(h http.HandlerFunc) http.HandlerFunc {
	return enableCORS(logMiddleware(timeoutMiddleware(h)))
}

func (s *RPCServer) Start() {
	mux := http.NewServeMux()

	mux.HandleFunc("/list", s.wrapHandler(s.handleList))
	mux.HandleFunc("/data/", s.wrapHandler(s.handleDataStructure))
	mux.HandleFunc("/structures", s.wrapHandler(s.handleGetStructures))
	mux.HandleFunc("/hash", s.wrapHandler(s.handleGetByHash))

	mux.HandleFunc("/health", s.wrapHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	}))

	s.server = &http.Server{
		Addr:         ":" + s.port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting RPC server on port %s", s.port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("RPC server failed: %v", err)
		}
	}()
}

func (s *RPCServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down RPC server...")
	return s.server.Shutdown(ctx)
}

func (s *RPCServer) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 0 {
		page = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	dataStructureID, _ := strconv.Atoi(r.URL.Query().Get("dsid"))

	messages, err := s.operator.db.GetAllMessages(dataStructureID, page, limit)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (s *RPCServer) handleDataStructure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/data/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	dataStructureID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid data structure ID", http.StatusBadRequest)
		return
	}

	switch parts[1] {
	case "list":
		s.handleFilteredList(w, r, dataStructureID)
	case "latest":
		s.handleLatest(w, r, dataStructureID)
	default:
		http.NotFound(w, r)
	}
}

func (s *RPCServer) handleFilteredList(w http.ResponseWriter, r *http.Request, dataStructureID int) {
	query := r.URL.Query()

	// Get all query params (field=value pairs)
	fieldFilters := make(map[string]string)
	for field, values := range query {
		if len(values) > 0 {
			fieldFilters[field] = values[0]
		}
	}

	page, _ := strconv.Atoi(query.Get("page"))
	limit, _ := strconv.Atoi(query.Get("limit"))
	if page < 0 {
		page = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	// For simplicity, we'll just use the first field filter
	var field, value string
	for f, v := range fieldFilters {
		field = f
		value = v
		break
	}

	messages, err := s.operator.db.GetMessagesByField(dataStructureID, field, value, page, limit)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (s *RPCServer) handleLatest(w http.ResponseWriter, r *http.Request, dataStructureID int) {
	query := r.URL.Query()
	field := query.Get("field")
	value := query.Get("value")

	threshold := s.operator.threshold()
	var msg Message
	var found bool
	var err error

	if field != "" && value != "" {
		msg, found, err = s.operator.db.GetLatestByField(dataStructureID, threshold, field, value)
	} else {
		msg, found, err = s.getLatestConfirmedMessage(dataStructureID, threshold)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	if !found {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func (s *RPCServer) getLatestConfirmedMessage(dataStructureID, threshold int) (Message, bool, error) {
	prefix := []byte(fmt.Sprintf("%s%d:", indexPrefix, dataStructureID))
	iter := s.operator.db.(*LevelDBDatabase).db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	if iter.Last() {
		for ; iter.Valid(); iter.Prev() {
			key := string(iter.Key())
			parts := strings.Split(key, ":")
			if len(parts) < 4 {
				continue
			}
			hash := parts[3]

			data, err := s.operator.db.(*LevelDBDatabase).db.Get([]byte(dataPrefix+hash), nil)
			if err != nil {
				continue
			}

			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			// Check signatures
			sigs, exists := s.operator.db.GetSignatures(msg.Hash)
			if exists && len(sigs) >= threshold {
				msg.Signatures = sigs
				return msg, true, nil
			}
		}
	}

	return Message{}, false, nil
}

func (s *RPCServer) handleGetByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, "Missing hash parameter", http.StatusBadRequest)
		return
	}

	data, structure, structureMeta, timestamp, exists := s.operator.db.GetData(hash)
	if !exists {
		http.Error(w, "Hash not found", http.StatusNotFound)
		return
	}

	signatures, _ := s.operator.db.GetSignatures(hash)

	msg := Message{
		Hash:              hash,
		Data:              data,
		DataStructure:     structure,
		DataStructureMeta: structureMeta,
		Signatures:        signatures,
		Timestamp:         timestamp,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func (s *RPCServer) handleGetStructures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ids, err := s.operator.db.GetDataStructures()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ids)
}
