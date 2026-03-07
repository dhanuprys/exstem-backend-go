package helper

import (
	"crypto/rand"
	"math/big"
)

// GenerateStudentPassword creates a random 6-character alphanumeric password
// using characters that are easy to distinguish (e.g., removing '0', 'O', '1', 'l', 'I').
func GenerateStudentPassword() (string, error) {
	const charset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	length := 6

	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		bytes[i] = charset[num.Int64()]
	}
	return string(bytes), nil
}
