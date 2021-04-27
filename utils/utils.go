package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
)

func AesCtrEncrypt(plainText, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := bytes.Repeat([]byte("1"), block.BlockSize())
	stream := cipher.NewCTR(block, iv)

	dst := make([]byte, len(plainText))
	stream.XORKeyStream(dst, plainText)

	return dst, nil
}


func AesCtrDecrypt(encryptData, key []byte) ([]byte, error) {
	return AesCtrEncrypt(encryptData, key)
}
