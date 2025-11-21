package api

import (
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/queue"
	"chat-app-backend/internal/websocket"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type RouteRegistrar func(mux *http.ServeMux, s *APIServer)

type APIServer struct {
	listenAddr          string
	requestQueueManager *queue.RequestQueueManager
	db                  *database.Database
	routeRegistrars     []RouteRegistrar
	handler             *websocket.Handler
	metrics             *metrics
}

func NewAPIServer(listenAddr string, rqm *queue.RequestQueueManager, db *database.Database, handler *websocket.Handler, registrars ...RouteRegistrar) *APIServer {

	return &APIServer{
		listenAddr:          listenAddr,
		requestQueueManager: rqm,
		db:                  db,
		handler:             handler,
		routeRegistrars:     registrars,
		metrics:             newMetrics(prometheus.DefaultRegisterer, listenAddr, rqm),
	}
}

func (s *APIServer) Run() {
	mux := http.NewServeMux()

	for _, reg := range s.routeRegistrars {
		reg(mux, s)
	}

	mux.Handle("/metrics", s.metrics.metricsHandler())

	fmt.Printf("Server listening on http://localhost%s\n", s.listenAddr)

	if err := http.ListenAndServe(s.listenAddr, s.metrics.instrument(mux)); err != nil {
		fmt.Printf("server stopped: %v\n", err)
	}
}

func (s *APIServer) Database() *database.Database {
	return s.db
}

func (s *APIServer) Handler() *websocket.Handler {
	return s.handler
}
