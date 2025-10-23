package websocket

type Room struct {
	Id      string               `json:"id"`
	Clients map[string]*WSClient `json:"clients"`
}

type WSMessage struct {
	Content   string `json:"content"`
	RoomID    string `json:"roomId"`
	Timestamp int64  `json:"timestamp"`
}

type JoinRoomReq struct {
	RoomID   string `json:"roomId"`
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

type RoomRes struct {
	ID string `json:"id"`
}

type ClientRes struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
