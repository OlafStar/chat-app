package main

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/router"
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/queue"
	"chat-app-backend/internal/websocket"
	"log"
)

func main() {
	queueManager := queue.NewRequestQueueManager(10, 10)
	db, err := database.NewDatabase()
	if err != nil {
		log.Fatalf("db init failed: %v", err)
	}

	hub := websocket.NewHub()
	go hub.Run()
	handler := websocket.NewHandler(hub)

	server := api.NewAPIServer(
		":83",
		queueManager,
		db,
		handler,
		router.UtilsRoutes("/api/ws/v1"),
		router.ConversationWebsocketRoutes("/api/ws/v1"),
	)

	handler.SubscribeToRedisChannels()

	server.Run()
}
