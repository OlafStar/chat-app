package websocket

import (
	"chat-app-backend/internal/env"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

var (
	upgrader    websocket.Upgrader
	redisClient *redis.Client
)

func init() {
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr:     env.Get("CHAT_REDIS_URL"),
		Password: env.Get("CHAT_REDIS_PASS"),
		DB:       0,
	})
}

type Handler struct {
	hub         *Hub
	redisClient *redis.Client
}

func NewHandler(h *Hub) *Handler {
	return &Handler{
		hub:         h,
		redisClient: redisClient,
	}
}

func (h *Handler) subscribeToRoomChannel(roomID string) {
	// Check if already subscribed
	if _, exists := h.hub.Rooms[roomID]; !exists {
		log.Printf("Room %s not found for subscription", roomID)
		return
	}

	log.Printf("Subscribing to Redis channel: %s", roomID)
	subscriber := h.redisClient.Subscribe(context.Background(), roomID)
	defer subscriber.Close()

	ch := subscriber.Channel()
	for msg := range ch {
		log.Printf("Received message from Redis channel '%s': %s", roomID, msg.Payload)

		h.hub.Broadcast <- &WSMessage{
			Content:   msg.Payload,
			RoomID:    roomID,
			Timestamp: time.Now().Unix(),
		}
	}
	log.Printf("Unsubscribed from Redis channel: %s", roomID)
}

func (h *Handler) CreateRoom(id string) {
	log.Printf("[WEBSOCKET_DEBUG]: CreateRoom Start")
	if _, exists := h.hub.Rooms[id]; exists {
		return
	}

	room := &Room{
		Id:      id,
		Clients: make(map[string]*WSClient),
	}

	h.hub.Rooms[id] = room

	// Ensure Redis subscription for the room only once
	go h.subscribeToRoomChannel(id)
	log.Printf("[WEBSOCKET_DEBUG]: CreateRoom End")
}

func (h *Handler) JoinRoom(w http.ResponseWriter, r *http.Request, roomId, userId string) {
	log.Printf("[WEBSOCKET_DEBUG]: JoinRoom Start")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if conn == nil {
		http.Error(w, "Error conn", http.StatusBadRequest)
		return
	}

	cl := &WSClient{
		Conn:     conn,
		Message:  make(chan *WSMessage, 10),
		ID:       userId,
		RoomID:   roomId,
		done:     make(chan struct{}),
		isClosed: false,
	}

	h.hub.Register <- cl

	go cl.keepAlive()
	go cl.writeMessage()
	go cl.readMessage(h.hub)
	log.Printf("[WEBSOCKET_DEBUG]: JoinRoom End")
}

func (h *Handler) GetRooms(w http.ResponseWriter, r *http.Request) {
	rooms := make([]RoomRes, 0)

	for _, room := range h.hub.Rooms {
		rooms = append(rooms, RoomRes{
			ID: room.Id,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rooms)
}

// func (h *Handler) Notify(createdMessage *dto.ChatMessageWithUserClientDTO, websocketRoomId string) {
// 	messageJSON, err := json.Marshal(createdMessage)
// 	if err != nil {
// 		log.Printf("Error marshaling notification message: %v", err)
// 		return
// 	}

// 	msg := WSMessage{
// 		Content:   string(messageJSON),
// 		RoomID:    websocketRoomId,
// 		Timestamp: createdMessage.Timestamp,
// 	}

// 	log.Printf("Publishing notification to Redis channel '%s': %s", websocketRoomId, msg.Content)
// 	if err := h.redisClient.Publish(context.Background(), websocketRoomId, msg.Content).Err(); err != nil {
// 		log.Printf("Error publishing notification to Redis: %v", err)
// 	}
// }

func (h *Handler) SubscribeToRedisChannels() {
	for _, room := range h.hub.Rooms {
		go h.subscribeToRoomChannel(room.Id)
	}
}
