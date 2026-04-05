// Package nanoid provides a wrapper around the go-nanoid library for generating secure, URL-friendly unique string IDs.
package nanoid

import gonanoid "github.com/matoous/go-nanoid/v2"

const (
	// Alphabet is the character set used for ID generation (A-Za-z0-9, 62 chars).
	Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	// IDLength is the length of generated IDs.
	IDLength = 10
)

// New generates a new nanoid of length IDLength using Alphabet.
func New() (string, error) {
	return gonanoid.Generate(Alphabet, IDLength)
}

// MustNew generates a new nanoid and panics on error.
func MustNew() string {
	id, err := New()
	if err != nil {
		panic(err)
	}
	return id
}
