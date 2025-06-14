// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package securecookie provides a cookie handler that encrypts its payload.
// Based on ideas from:
// https://moroz.dev/blog/secure-cookie-library-in-go-from-scratch/
package securecookie

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	maxSize       = 4096 // maximum cookie size
	timestampSize = 8
)

const (
	keySize   = chacha20poly1305.KeySize
	nonceSize = chacha20poly1305.NonceSizeX
	overhead  = chacha20poly1305.Overhead
)

var (
	// ErrMsgTooShort is the error for a payload that's too short.
	ErrMsgTooShort = errors.New("message too short")
	// ErrMsgTooLong is returned when the message is too big.
	ErrMsgTooLong = errors.New("message too long")
	// ErrExpired is returned when the payload has expired.
	ErrExpired = errors.New("message expired")
)

// Key is an 256bit key.
type Key [keySize]byte

// store provided a content encoder and decoder.
type store struct {
	key Key
}

// Handler is the message and cookie handler.
type Handler struct {
	store *store

	name       string
	path       string
	maxAge     int
	enforceTTL bool

	now func() time.Time
}

// Option is a function to set options to a [Handler].
type Option func(h *Handler)

// encode encoded the given data using the store's key.
func (s *store) encode(data []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize, len(data)+overhead+nonceSize)
	rand.Read(nonce)

	aead, err := chacha20poly1305.NewX(s.key[:])
	if err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, data, nil), nil
}

// decode decodes the given data using the store's key.
func (s *store) decode(data []byte) ([]byte, error) {
	if len(data) < nonceSize+overhead {
		return nil, fmt.Errorf("%w (got %d want %d)", ErrMsgTooShort, len(data), nonceSize+overhead)
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	aead, err := chacha20poly1305.NewX(s.key[:])
	if err != nil {
		return nil, err
	}

	return aead.Open(nil, nonce, ciphertext, nil)
}

// NewHandler returns a new [Handler] with an associated [Encoder]
// using the provided key.
func NewHandler(key Key, options ...Option) *Handler {
	h := &Handler{
		store:      &store{key: key},
		name:       "session",
		path:       "/",
		maxAge:     86400 * 30,
		enforceTTL: true,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}

	for _, fn := range options {
		fn(h)
	}

	return h
}

// WithName is an [Option] that sets the cookie's name.
func WithName(name string) Option {
	return func(h *Handler) {
		h.name = name
	}
}

// WithPath is an [Option] that sets the cookie's path.
func WithPath(path string) Option {
	return func(h *Handler) {
		h.path = path
	}
}

// WithMaxAge is an [Option] that sets the cookie's max age, in seconds.
func WithMaxAge(maxAge int) Option {
	return func(h *Handler) {
		h.maxAge = maxAge
	}
}

// WithTTL is an [Option] that sets the payload's TTL enforcement.
func WithTTL(ttl bool) Option {
	return func(h *Handler) {
		h.enforceTTL = ttl
	}
}

// Encode encodes the provided data, prepended with a timestamp.
func (h *Handler) Encode(data any) ([]byte, error) {
	var buf bytes.Buffer

	// 1. write the timestamp
	if err := binary.Write(&buf, binary.LittleEndian, h.now().Unix()); err != nil {
		return nil, err
	}

	// 2. write the content
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		return nil, err
	}

	// 3. encode
	return h.store.encode(buf.Bytes())
}

// Decode decodes the provided data into "dst".
// Timestamp validation takes place at this moment and this function
// returns [ErrExpired] when the data has expired.
func (h *Handler) Decode(data []byte, dst any) error {
	// 1. decode the message
	msg, err := h.store.decode(data)
	if err != nil {
		return err
	}

	if len(msg) < timestampSize {
		return fmt.Errorf("%w (got %d, want at least %d)", ErrMsgTooShort, len(msg), timestampSize)
	}

	// 2. retrieve and check timestamp
	var ts int64
	if err = binary.Read(bytes.NewReader(msg[:timestampSize]), binary.LittleEndian, &ts); err != nil {
		return err
	}

	if h.enforceTTL && time.Unix(ts, 0).Add(time.Duration(h.maxAge)*time.Second).Before(h.now()) {
		return ErrExpired
	}

	// 3. get the payload
	return json.Unmarshal(msg[timestampSize:], dst)
}

// Load decodes the cookie into "payload".
// It loads the cookie by its name, decodes its base64 value
// and then calls [Handler.Decode] on it.
func (h *Handler) Load(r *http.Request, payload any) error {
	c, err := r.Cookie(h.name)
	if err != nil {
		return err
	}

	encoded, err := base64.URLEncoding.DecodeString(c.Value)
	if err != nil {
		return err
	}
	return h.Decode(encoded, payload)
}

// Save writes a new cookie based on "payload" to an [http.ResponseWriter].
// It encodes the payload using [Handler.Encode], encodes it to base64
// and then writes everything into a new cookie.
// If the encoding value is longer than 4096, it returns [ErrMsgTooLong].
func (h *Handler) Save(w http.ResponseWriter, r *http.Request, payload any) error {
	encoded, err := h.Encode(payload)
	if err != nil {
		return err
	}

	cookie := h.newCookie(r)
	cookie.Value = base64.URLEncoding.EncodeToString(encoded)
	if len(cookie.Value) > maxSize {
		return ErrMsgTooLong
	}

	http.SetCookie(w, cookie)
	return nil
}

// Delete sets a cookie with MaxAge -1 to the [http.ResponseWriter],
// which removes it from the browser.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	cookie := h.newCookie(r)
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0).UTC()
	http.SetCookie(w, cookie)
}

func (h *Handler) newCookie(r *http.Request) *http.Cookie {
	c := &http.Cookie{
		Name:     h.name,
		Path:     h.path,
		MaxAge:   h.maxAge,
		Secure:   r.URL.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	}

	if c.MaxAge > 0 {
		c.Expires = h.now().Add(time.Duration(h.maxAge) * time.Second)
	}

	return c
}
