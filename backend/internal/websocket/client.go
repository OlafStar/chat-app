package websocket

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	Conn     *websocket.Conn
	Message  chan *WSMessage
	ID       string
	RoomID   string
	done     chan struct{} // Signal for coordinating goroutine shutdown
	mu       sync.Mutex    // Mutex for connection access
	isClosed bool          // Flag to track connection state
}

func (cl *WSClient) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cl.done:
			return
		case <-ticker.C:
			cl.mu.Lock()
			if cl.isClosed {
				cl.mu.Unlock()
				return
			}
			err := cl.Conn.WriteMessage(websocket.PingMessage, nil)
			cl.mu.Unlock()

			if err != nil {
				log.Printf("Ping error for client %s: %v", cl.ID, err)
				return
			}
		}
	}
}

func (cl *WSClient) writeMessage() {
	defer func() {
		cl.mu.Lock()
		cl.isClosed = true
		cl.Conn.Close()
		cl.mu.Unlock()
	}()

	for {
		select {
		case <-cl.done:
			return
		case msg, ok := <-cl.Message:
			if !ok {
				log.Printf("Client %s message channel closed", cl.ID)
				return
			}

			cl.mu.Lock()
			if cl.isClosed {
				cl.mu.Unlock()
				return
			}
			err := cl.Conn.WriteJSON(msg)
			cl.mu.Unlock()

			if err != nil {
				log.Printf("Error sending message to client %s: %v", cl.ID, err)
				return
			}
		}
	}
}

func (cl *WSClient) readMessage(hub *Hub) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in readMessage: %v", r)
		}

		if cl.done != nil {
			close(cl.done)
		}

		hub.Unregister <- cl
		log.Printf("Client %s disconnected from room %s", cl.ID, cl.RoomID)
	}()

	cl.Conn.SetReadLimit(512 * 1024) // Set a reasonable read limit

	for {
		_, message, err := cl.Conn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*websocket.CloseError); ok {
				if closeErr.Code == websocket.CloseNormalClosure ||
					closeErr.Code == websocket.CloseGoingAway ||
					closeErr.Code == websocket.CloseNoStatusReceived {
					break
				}
			}
			log.Printf("Error reading message from client %s: %v", cl.ID, err)
			break
		}

		hub.Broadcast <- &WSMessage{
			Content:   string(message),
			RoomID:    cl.RoomID,
			Timestamp: time.Now().Unix(),
		}
	}
}
