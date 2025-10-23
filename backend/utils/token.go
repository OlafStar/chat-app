package utils

import "github.com/google/uuid"

func CreateToken() string {
	firstUUID, err := uuid.NewUUID()

	if err != nil {
		return ""
	}
	
	secondUUID, err := uuid.NewUUID()

	if err != nil {
		return ""
	}

	token := firstUUID.String() + secondUUID.String()

	return token
}