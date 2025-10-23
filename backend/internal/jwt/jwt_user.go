package jwt

import "golang.org/x/crypto/bcrypt"

func NewUser(user RegisterUser) (User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
	if err != nil {
		return User{}, err
	}

	return User{
		Email:        user.Email,
		PasswordHash: string(hashedPassword),
	}, nil
}

func ValidatePassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}