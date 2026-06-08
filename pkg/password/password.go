package password

import (
	"crypto/rand"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const (
	upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars   = "abcdefghijklmnopqrstuvwxyz"
	digitChars   = "0123456789"
	specialChars = "@#$%&*!"
	allChars     = upperChars + lowerChars + digitChars + specialChars
)

func Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func Verify(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func Generate(length int) (string, error) {
	if length < 8 {
		length = 8
	}

	password := make([]byte, length)
	required := []string{upperChars, lowerChars, digitChars, specialChars}

	for i, chars := range required {
		ch, err := randomChar(chars)
		if err != nil {
			return "", err
		}
		password[i] = ch
	}

	for i := len(required); i < length; i++ {
		ch, err := randomChar(allChars)
		if err != nil {
			return "", err
		}
		password[i] = ch
	}

	for i := len(password) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		password[i], password[j.Int64()] = password[j.Int64()], password[i]
	}

	return string(password), nil
}

func randomChar(chars string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
	if err != nil {
		return 0, err
	}
	return chars[n.Int64()], nil
}
