package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultCost is the default bcrypt cost
	DefaultCost = 12
)

// HashPassword generates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ComparePassword compares a password with a hash
// Returns true if the password matches the hash
func ComparePassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// ValidatePasswordStrength validates password strength requirements
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}
