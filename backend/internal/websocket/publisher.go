package websocket

import (
	"context"
	"encoding/json"
	"fmt"
)

func Publish(roomID string, payload interface{}) error {
	if roomID == "" {
		return fmt.Errorf("websocket publish: roomID required")
	}
	if redisClient == nil {
		return fmt.Errorf("websocket publish: redis client not initialised")
	}

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("websocket publish: marshal payload: %w", err)
	}

	if err := redisClient.Publish(context.Background(), roomID, string(messageJSON)).Err(); err != nil {
		return fmt.Errorf("websocket publish: redis publish: %w", err)
	}
	return nil
}
