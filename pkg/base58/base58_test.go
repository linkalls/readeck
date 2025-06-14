// SPDX-FileCopyrightText: © 2013-2015 The btcsuite developers
//
// SPDX-License-Identifier: ISC

package base58_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/pkg/base58"
)

func h2b(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func uuid2b(s string) []byte {
	u, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return u[:]
}

func FuzzDecode(f *testing.F) {
	for range 50 {
		l, _ := rand.Int(rand.Reader, big.NewInt(48))
		l = l.Add(l, big.NewInt(1))
		data := make([]byte, l.Int64())
		rand.Read(data)
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, a []byte) {
		s := base58.EncodeToString(a)
		decoded, err := base58.DecodeString(s)
		if err != nil {
			t.Fatalf("unexpected error %s", err.Error())
		}
		if !bytes.Equal(decoded, a) {
			t.Fatalf("got: %q, wanted: %q", decoded, a)
		}
	})
}

func TestB58(t *testing.T) {
	tests := []struct {
		decoded []byte
		encoded string
	}{
		// String inputs
		{[]byte(""), ""},
		{[]byte(" "), "Z"},
		{[]byte("-"), "n"},
		{[]byte("0"), "q"},
		{[]byte("1"), "r"},
		{[]byte("-1"), "4SU"},
		{[]byte("11"), "4k8"},
		{[]byte("abc"), "ZiCa"},
		{[]byte("1234598760"), "3mJr7AoUXx2Wqd"},
		{[]byte("abcdefghijklmnopqrstuvwxyz"), "3yxU3u1igY8WkgtjK92fbJQCd4BZiiT1v25f"},
		{[]byte("00000000000000000000000000000000000000000000000000000000000000"), "3sN2THZeE9Eh9eYrwkvZqNstbHGvrxSAM7gXUXvyFQP8XvQLUqNCS27icwUeDT7ckHm4FUHM2mTVh1vbLmk7y"},

		// Hex inputs.
		{h2b("61"), "2g"},
		{h2b("626262"), "a3gV"},
		{h2b("636363"), "aPEr"},
		{h2b("73696d706c792061206c6f6e6720737472696e67"), "2cFupjhnEsSn59qHXstmK2ffpLv2"},
		{h2b("00eb15231dfceb60925886b67d065299925915aeb172c06647"), "1NS17iag9jJgTHD1VXjvLCEnZuQ3rJDE9L"},
		{h2b("516b6fcd0f"), "ABnLTmg"},
		{h2b("bf4f89001e670274dd"), "3SEo3LWLoPntC"},
		{h2b("572e4794"), "3EFU7m"},
		{h2b("ecac89cad93923c02321"), "EJDM8drfXA6uyA"},
		{h2b("10c8511e"), "Rt5zm"},
		{h2b("00000000000000000000"), "1111111111"},

		// some uuids
		{uuid2b("dc5eb8be-935e-48b4-a875-bb6d56ab9484"), "UDJwD3JQyUzSXnDfcPwAPM"},
		{uuid2b("b27d7e58-0c7e-4417-9840-9e68d75ac596"), "P3N2J7smMHsDXtdXWsvmLH"},
		{uuid2b("fc9ab497-af12-4fbc-9ef3-346ffea2e599"), "YCB6ukdJeQQ8jbLMNauV9A"},
		{uuid2b("ac95ae33-3c18-4ab3-b8c8-526132c55218"), "NK4s4dXFhbwtWwKH9yXCFq"},
		{uuid2b("3f865103-a1da-45cc-b894-b9e1dec58eec"), "8qyDaaR9jNMbz4rYWTk2zf"},
		{uuid2b("a318acb5-3ee9-4a61-8975-730e27549e7b"), "M97QsaYJkj7Tip6F9WLUT4"},
		{uuid2b("80dd05ff-67cf-4de4-b240-5d3db038f62c"), "Guvywu2ufH7AMMUUsMPU6F"},
		{uuid2b("21842526-3c7d-4e99-816e-43f7dd37350a"), "593fNsVRSkgnKrib8SFPHs"},
		{uuid2b("b34e9b20-2bb5-4a09-9d99-bf4f20b649b2"), "P9DLmQXawztNSGtkrsAugd"},
		{uuid2b("a2449574-ac5b-4511-87d9-99bfd001e3a7"), "M3BG6Jn4pMENMkTfXk5a7Q"},
		{uuid2b("77bef40c-13d4-46a2-9116-ad7092c680e8"), "FndaZicXRsG5zSqeFFoFSw"},
		{uuid2b("9159cde1-627f-4b85-9e43-da4a05f1be44"), "Jx1snke6pW2RtHkNnjDCQb"},
		{uuid2b("259e84fe-2555-4367-a248-2d42bd3f96c8"), "5eS4da452PjehJ2DdpnnGP"},
		{uuid2b("6357b08e-6c64-4834-bb90-a917d572bb27"), "DGVzkMz9nykcjcQNZLdRUS"},
		{uuid2b("03630f49-289d-4a95-9f68-c07531319208"), "RFwimz6svzb8tTeTMBXiw"},
		{uuid2b("6ba37d99-6e30-42a9-9f6a-63d284c7b460"), "EHvCo2reWqyVXMXSMLBFXV"},
		{uuid2b("9e378a71-3df5-460e-b56c-5db68f9289f9"), "LYAVWbvkvBaeDA8U9N4Gba"},
		{uuid2b("7ffa494b-73de-4da2-95f0-8411462bf899"), "Gob4jwjbav6BVn4DKfdmHA"},
		{uuid2b("451f6ec9-d04a-40ac-83d6-9598f17303e4"), "9Y4gKd7pkQTseaLS7sUYy1"},
		{uuid2b("17e5c85f-2f96-48c9-8cf4-4034d845aedd"), "3xA5raxdqfTHkPvPKqfQwv"},
	}

	for i, test := range tests {
		if s := base58.EncodeToString(test.decoded); s != test.encoded {
			t.Errorf("Encode test %d failed: got: %q, wanted: %q", i, s, test.encoded)
		}

		b, err := base58.DecodeString(test.encoded)
		if err != nil {
			t.Errorf("unexpected error in test %d: %s", i, err.Error())
		}
		if !bytes.Equal(b, test.decoded) {
			t.Errorf("Decode test %d failed: got: %x, wanted: %x", i, b, test.decoded)
		}
	}
}

func TestDecodeError(t *testing.T) {
	_, err := base58.DecodeString("🙂")
	if err == nil || !errors.Is(err, base58.ErrInvalidChar) {
		t.Errorf("wanted an error, got: %v", err)
	}
}

func TestNewEncodingErrors(t *testing.T) {
	tests := []struct {
		encoder string
		err     string
	}{
		{
			"aa",
			"encoding alphabet is not 58-bytes long",
		},
		{
			"\n23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz",
			"encoding alphabet contains newline character",
		},
		{
			"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnZpqrstuvwxyz",
			"encoding alphabet includes duplicate symbols",
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			defer func() {
				r := recover()
				if r != test.err {
					t.Fatalf("wanted: %q, got: %q", test.err, r)
				}
			}()

			base58.NewEncoding(test.encoder)
		})
	}
}

func FuzzUUID(f *testing.F) {
	for range 5 {
		data := base58.NewUUID()
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, a string) {
		decoded, err := base58.DecodeUUID(a)
		if err != nil {
			t.Fatalf("unexpected error %s", err.Error())
		}
		encoded := base58.EncodeUUID(decoded)
		if encoded != a {
			t.Fatalf("got: %q, wanted: %q", decoded, a)
		}
	})
}

func TestEncodeUUID(t *testing.T) {
	tests := []struct {
		decoded []byte
		encoded string
	}{
		// some uuids
		{uuid2b("dc5eb8be-935e-48b4-a875-bb6d56ab9484"), "UDJwD3JQyUzSXnDfcPwAPM"},
		{uuid2b("b27d7e58-0c7e-4417-9840-9e68d75ac596"), "P3N2J7smMHsDXtdXWsvmLH"},
		{uuid2b("fc9ab497-af12-4fbc-9ef3-346ffea2e599"), "YCB6ukdJeQQ8jbLMNauV9A"},
		{uuid2b("ac95ae33-3c18-4ab3-b8c8-526132c55218"), "NK4s4dXFhbwtWwKH9yXCFq"},
		{uuid2b("3f865103-a1da-45cc-b894-b9e1dec58eec"), "8qyDaaR9jNMbz4rYWTk2zf"},
		{uuid2b("a318acb5-3ee9-4a61-8975-730e27549e7b"), "M97QsaYJkj7Tip6F9WLUT4"},
		{uuid2b("80dd05ff-67cf-4de4-b240-5d3db038f62c"), "Guvywu2ufH7AMMUUsMPU6F"},
		{uuid2b("21842526-3c7d-4e99-816e-43f7dd37350a"), "593fNsVRSkgnKrib8SFPHs"},
		{uuid2b("b34e9b20-2bb5-4a09-9d99-bf4f20b649b2"), "P9DLmQXawztNSGtkrsAugd"},
		{uuid2b("a2449574-ac5b-4511-87d9-99bfd001e3a7"), "M3BG6Jn4pMENMkTfXk5a7Q"},
		{uuid2b("77bef40c-13d4-46a2-9116-ad7092c680e8"), "FndaZicXRsG5zSqeFFoFSw"},
		{uuid2b("9159cde1-627f-4b85-9e43-da4a05f1be44"), "Jx1snke6pW2RtHkNnjDCQb"},
		{uuid2b("259e84fe-2555-4367-a248-2d42bd3f96c8"), "5eS4da452PjehJ2DdpnnGP"},
		{uuid2b("6357b08e-6c64-4834-bb90-a917d572bb27"), "DGVzkMz9nykcjcQNZLdRUS"},
		{uuid2b("03630f49-289d-4a95-9f68-c07531319208"), "RFwimz6svzb8tTeTMBXiw"},
		{uuid2b("6ba37d99-6e30-42a9-9f6a-63d284c7b460"), "EHvCo2reWqyVXMXSMLBFXV"},
		{uuid2b("9e378a71-3df5-460e-b56c-5db68f9289f9"), "LYAVWbvkvBaeDA8U9N4Gba"},
		{uuid2b("7ffa494b-73de-4da2-95f0-8411462bf899"), "Gob4jwjbav6BVn4DKfdmHA"},
		{uuid2b("451f6ec9-d04a-40ac-83d6-9598f17303e4"), "9Y4gKd7pkQTseaLS7sUYy1"},
		{uuid2b("17e5c85f-2f96-48c9-8cf4-4034d845aedd"), "3xA5raxdqfTHkPvPKqfQwv"},
	}

	for i, test := range tests {
		u := uuid.UUID(test.decoded)

		if s := base58.EncodeUUID(u); s != test.encoded {
			t.Errorf("Encode test %d failed: got: %q, wanted: %q", i, s, test.encoded)
		}

		decoded, err := base58.DecodeUUID(test.encoded)
		if err != nil {
			t.Errorf("unexpected error in test %d: %s", i, err.Error())
		}
		if !bytes.Equal(decoded[:], test.decoded) {
			t.Errorf("Decode test %d failed: got: %x, wanted: %x", i, decoded, test.decoded)
		}
	}
}

func TestDecodeUUIDError(t *testing.T) {
	_, err := base58.DecodeUUID("abcd")
	if err == nil || !errors.Is(err, base58.ErrInvalidLenght) {
		t.Errorf("wanted an error, got: %v", err)
	}

	_, err = base58.DecodeUUID("🙂")
	if err == nil || !errors.Is(err, base58.ErrInvalidChar) {
		t.Errorf("wanted an error, got: %v", err)
	}
}

func BenchmarkB32EncodeToString(b *testing.B) {
	id := uuid.New()
	b.ResetTimer()

	for b.Loop() {
		base32.HexEncoding.EncodeToString(id[:])
	}
}

func BenchmarkB32DecodeString(b *testing.B) {
	id := uuid.New()
	encoded := base32.HexEncoding.EncodeToString(id[:])
	var err error
	b.ResetTimer()

	for b.Loop() {
		_, err = base32.HexEncoding.DecodeString(encoded)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkEncodeToString(b *testing.B) {
	id := uuid.New()
	b.ResetTimer()

	for b.Loop() {
		base58.EncodeToString(id[:])
	}
}

func BenchmarkDecodeString(b *testing.B) {
	id := uuid.New()
	encoded := base58.EncodeToString(id[:])
	var err error
	b.ResetTimer()

	for b.Loop() {
		_, err = base58.DecodeString(encoded)
		if err != nil {
			panic(err)
		}
	}
}
