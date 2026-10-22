package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

const fileSecretMagic = "m365cli-secret-v1\x00"

func fileCipher(salt []byte) (cipher.AEAD, error) {
	password, ok := keyringPassword()
	if !ok || password == "" {
		return nil, errors.New("file keyring requires a non-empty M365CLI_KEYRING_PASSWORD")
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, 210000, 32)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptFileSecret(key string, value []byte) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	aead, err := fileCipher(salt)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte(fileSecretMagic), salt...)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, value, []byte(key)), nil
}

func decryptFileSecret(key string, raw []byte) ([]byte, error) {
	if len(raw) < len(fileSecretMagic)+16 || string(raw[:len(fileSecretMagic)]) != fileSecretMagic {
		return nil, errors.New("unrecognized or legacy plaintext keyring entry; sign in again using the encrypted keyring")
	}
	offset := len(fileSecretMagic)
	aead, err := fileCipher(raw[offset : offset+16])
	if err != nil {
		return nil, err
	}
	offset += 16
	if len(raw) < offset+aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("truncated encrypted keyring entry")
	}
	plaintext, err := aead.Open(nil, raw[offset:offset+aead.NonceSize()], raw[offset+aead.NonceSize():], []byte(key))
	if err != nil {
		return nil, errors.New("cannot decrypt keyring entry: incorrect password or damaged entry")
	}
	return plaintext, nil
}
