package websocket

type Hub struct {
	Rooms      map[string]*Room
	Register   chan *WSClient
	Unregister chan *WSClient
	Broadcast  chan *WSMessage
}

func NewHub() *Hub {
	return &Hub{
		Rooms:      make(map[string]*Room),
		Register:   make(chan *WSClient),
		Unregister: make(chan *WSClient),
		Broadcast:  make(chan *WSMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			room, ok := h.Rooms[client.RoomID]
			if !ok {
				// Handle the case where the room does not exist
				// For example, create a new room or log an error
				// In this example, we simply log and ignore the client registration
				continue
			}
			room.Clients[client.ID] = client
			incConnections()

		case client := <-h.Unregister:
			room, ok := h.Rooms[client.RoomID]
			if !ok {
				// Handle the case where the room does not exist
				// Just log a message and ignore the client unregistration
				continue
			}
			if _, ok := room.Clients[client.ID]; ok {
				delete(room.Clients, client.ID)
				close(client.Message)
				decConnections()
			}

		case message := <-h.Broadcast:
			room, ok := h.Rooms[message.RoomID]
			if !ok {
				// Handle the case where the room does not exist
				// Just log a message and ignore the broadcast
				continue
			}
			delivered := 0
			for _, client := range room.Clients {
				select {
				case client.Message <- message:
					delivered++
				default:
					close(client.Message)
					delete(room.Clients, client.ID)
					decConnections()
				}
			}
			if delivered > 0 {
				addDelivered(delivered)
			}
		}
	}
}
