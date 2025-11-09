package utils

import (
	"strings"

	"github.com/google/uuid"
)

// GenerateAPIKey returns a new tenant API key using a stable pingy_ prefix
// followed by the uppercase UUID without dashes. Keys issued during tenant
// registration use the same format so rotations stay compatible.
func GenerateAPIKey() string {
	key := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))
	return "pingy_" + key
}
