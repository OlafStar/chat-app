package main

import (
	"chat-app-backend/internal/api"
	"chat-app-backend/internal/api/router"
	"chat-app-backend/internal/database"
	"chat-app-backend/internal/queue"
	"log"
)

func main() {
	queue := queue.NewRequestQueueManager(10, 10)
	db, err := database.NewDatabase()

	if err != nil {
		log.Fatalf("db init failed: %v", err)
	}

	server := api.NewAPIServer(
		":81",
		queue,
		db,
		nil,
		router.UtilsRoutes("/api/client/v1"),
	)

	server.Run()
}
