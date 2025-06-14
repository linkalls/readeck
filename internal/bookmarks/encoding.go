// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"

	"golang.org/x/crypto/blake2b"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/base58"
)

// EncodeID returns an 160-bit base58 encoded ID and timestamp.
// This provide stateless expiration.
//
// Message 64-bit long and contains the ID and the expiration timestamp.
// It's then MACed with a salted key.
//
// Finale message is:
//
// ---------------------------------------------
// | 32-bit | 64-bit MAC     | 32-bit | 32-bit |
// | salt   |                | ID     | TS     |
// ---------------------------------------------
//
// This returns a base58 encoded string.
func EncodeID(id uint64, expires time.Time) (string, error) {
	msg := make([]byte, 20)
	binary.LittleEndian.PutUint32(msg[12:16], uint32(id))

	// Timestamp is expressed in minutes after rounding.
	// This gives us plenty of time in a 32-bit unsigned integer.
	binary.LittleEndian.PutUint32(msg[16:], uint32(expires.Round(time.Minute).Unix()/60))

	salt := make([]byte, 4)
	rand.Read(salt)

	k, err := configs.Keys.Expand("bookmark_share_"+string(salt), 32)
	if err != nil {
		return "", err
	}

	h, err := blake2b.New(8, k)
	if err != nil {
		return "", err
	}

	h.Write(msg[12:])

	copy(msg[:4], salt)
	copy(msg[4:12], h.Sum(nil))

	return base58.EncodeToString(msg), nil
}

// DecodeID deciphers a base58 encoded value
// into a timestamp and a unsigned integer.
func DecodeID(text string) (uint64, time.Time, error) {
	// Load the base64 encoded value. It must be exactly 16 bytes.
	msg, err := base58.DecodeString(text)
	if err != nil {
		return 0, time.Time{}, err
	}

	if len(msg) != 20 {
		return 0, time.Time{}, errors.New("invalid size")
	}

	// Get mac and salt
	salt := msg[:4]
	k, _ := configs.Keys.Expand("bookmark_share_"+string(salt), 32)

	// Verify MAC
	h, err := blake2b.New(8, k)
	if err != nil {
		return 0, time.Time{}, err
	}
	h.Write(msg[12:])
	if subtle.ConstantTimeCompare(h.Sum(nil), msg[4:12]) != 1 {
		return 0, time.Time{}, errors.New("invalid data")
	}

	// Unpack content
	return uint64(binary.LittleEndian.Uint32(msg[12:16])),
		time.Unix(int64(binary.LittleEndian.Uint32(msg[16:]))*60, 0),
		nil
}
