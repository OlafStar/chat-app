package api

import (
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/queue"
	"chat-app-backend/internal/websocket"
	"fmt"
	"net/http"
)

type RouteRegistrar func(mux *http.ServeMux, s *APIServer)

type APIServer struct {
	listenAddr          string
	requestQueueManager *queue.RequestQueueManager
	db                  *database.Database
	routeRegistrars     []RouteRegistrar
	handler             *websocket.Handler
}

func NewAPIServer(listenAddr string, rqm *queue.RequestQueueManager, db *database.Database, handler *websocket.Handler, registrars ...RouteRegistrar) *APIServer {

	return &APIServer{
		listenAddr:          listenAddr,
		requestQueueManager: rqm,
		db:                  db,
		handler:             handler,
		routeRegistrars:     registrars,
	}
}

func (s *APIServer) Run() {
	mux := http.NewServeMux()

	for _, reg := range s.routeRegistrars {
		reg(mux, s)
	}

	fmt.Printf("Server listening on http://localhost%s\n", s.listenAddr)

	http.ListenAndServe(s.listenAddr, mux)
}

func (s *APIServer) Database() *database.Database {
	return s.db
}

func (s *APIServer) Handler() *websocket.Handler {
	return s.handler
}
