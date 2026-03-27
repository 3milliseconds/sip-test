package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/nightowl/sip-test/internal/report"
	"github.com/nightowl/sip-test/internal/testrunner"
)

//go:embed all:static
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server is the HTTP server for the SIP test tool.
type Server struct {
	runner  *testrunner.Runner
	addr    string
	wsConns map[*websocket.Conn]bool
	wsMu    sync.Mutex
}

// New creates a new HTTP server.
func New(addr string, runner *testrunner.Runner) *Server {
	s := &Server{
		runner:  runner,
		addr:    addr,
		wsConns: make(map[*websocket.Conn]bool),
	}

	// Wire up status updates to WebSocket broadcast
	runner.SetOnUpdate(func(status testrunner.RunStatus) {
		s.broadcast(map[string]any{
			"type": "status",
			"data": status,
		})
	})

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/tests/run", s.handleRunTest)
		r.Post("/tests/run-config", s.handleRunConfig)
		r.Get("/tests/running", s.handleGetRunning)
		r.Get("/reports", s.handleGetReports)
	})

	// WebSocket
	r.Get("/ws", s.handleWebSocket)

	// Serve embedded static files
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	r.Handle("/*", http.FileServer(http.FS(staticSub)))

	log.Printf("Starting web server on %s", s.addr)
	return http.ListenAndServe(s.addr, r)
}

func (s *Server) handleRunTest(w http.ResponseWriter, r *http.Request) {
	var tc testrunner.TestConfig
	if err := json.NewDecoder(r.Body).Decode(&tc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Run test in background
	go func() {
		ctx := context.Background()
		tr, err := s.runner.RunTest(ctx, tc)
		if err != nil {
			log.Printf("Test error: %v", err)
		}
		if tr != nil {
			s.broadcast(map[string]any{
				"type": "result",
				"data": tr,
			})
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{"status": "started", "test": tc.Name})
}

func (s *Server) handleRunConfig(w http.ResponseWriter, r *http.Request) {
	var cfg testrunner.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go func() {
		ctx := context.Background()
		reports, _ := s.runner.RunAll(ctx, &cfg)
		suite := report.NewSuite("API Test Run", reports)
		s.broadcast(map[string]any{
			"type": "suite_complete",
			"data": suite,
		})
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{
		"status": "started",
		"tests":  fmt.Sprintf("%d", len(cfg.Tests)),
	})
}

func (s *Server) handleGetRunning(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.runner.GetRunning())
}

func (s *Server) handleGetReports(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.runner.GetResults())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	s.wsMu.Lock()
	s.wsConns[conn] = true
	s.wsMu.Unlock()

	// Keep connection alive, clean up on close
	defer func() {
		s.wsMu.Lock()
		delete(s.wsConns, conn)
		s.wsMu.Unlock()
		conn.Close()
	}()

	// Configure pong handler to reset the read deadline on each pong
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Ping ticker to keep the connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		for range pingTicker.C {
			s.wsMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
			s.wsMu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// Read loop (detect disconnect)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) broadcast(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	for conn := range s.wsConns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(s.wsConns, conn)
		}
	}
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
