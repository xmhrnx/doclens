package ui

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/xmhrnx/giode"
)

const pinKey = "doclens.pin"

// savePIN stores a salted hash of the PIN in the platform's secure
// storage. It may block: call it on a goroutine.
func savePIN(pin string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	return giode.SecureStore.Set(pinKey, []byte(hex.EncodeToString(salt)+":"+hashPIN(salt, pin)))
}

// checkPIN reports whether pin matches the stored one. It may block.
func checkPIN(pin string) (bool, error) {
	data, err := giode.SecureStore.Get(pinKey)
	if err != nil {
		return false, err
	}
	saltHex, want, ok := strings.Cut(string(data), ":")
	if !ok {
		return false, errors.New("the stored PIN is damaged")
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(hashPIN(salt, pin)), []byte(want)) == 1, nil
}

// clearPIN forgets the PIN. It may block.
func clearPIN() error {
	err := giode.SecureStore.Delete(pinKey)
	if errors.Is(err, giode.ErrNotFound) {
		return nil
	}
	return err
}

func hashPIN(salt []byte, pin string) string {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(pin))
	return hex.EncodeToString(h.Sum(nil))
}
