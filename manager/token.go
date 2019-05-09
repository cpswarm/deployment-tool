package main

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"log"
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
// https://stackoverflow.com/questions/32349807/how-can-i-generate-a-random-int-using-the-crypto-rand-package
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GenerateRandomToken(n int) (token, hash string, err error) {
	b, err := GenerateRandomBytes(n / 2)
	if err != nil {
		return "", "", fmt.Errorf("error generating random bytes: %s", err)
	}
	x := fmt.Sprintf("%x", b)
	c := n / 3
	token = x[:c] + "-" + x[c:c*2] + "-" + x[c*2:]
	hash, err = HashToken(token)
	if err != nil {
		return "", "", fmt.Errorf("error hashing token: %s", err)
	}
	log.Println("GenerateRandomToken", token, hash)
	return token, hash, nil
}

func HashToken(token string) (hash string, err error) {
	h := sha512.New()
	h.Write([]byte(token))
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
