package app

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
)

func generateToken(username string, aead cipher.AEAD) string {
	cipherText := make([]byte, aead.NonceSize()+len(username)+aead.Overhead())
	rand.Read(cipherText[:aead.NonceSize()])
	nonce := cipherText[:aead.NonceSize():aead.NonceSize()]
	cipherText = aead.Seal(cipherText[:len(nonce)], nonce, []byte(username), nil)
	return base64.StdEncoding.EncodeToString(cipherText)
}

func parseUserNameFromToken(token string, aead cipher.AEAD) (string, bool) {
	cipherText, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", false
	}
	nonce := cipherText[:aead.NonceSize():aead.NonceSize()]
	plainText, err := aead.Open(nil, nonce, cipherText[aead.NonceSize():], nil)
	if err != nil {
		return "", false
	}
	return string(plainText), true
}
