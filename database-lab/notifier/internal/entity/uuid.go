package entity

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Раскладка UUIDv7 по RFC 9562: старшие 48 бит — unix-ms, дальше версия,
// вариант и случайный хвост.
const (
	shift6         = 40
	shift5         = 32
	shift4         = 24
	shift3         = 16
	shift2         = 8
	uuidVersion7   = 0x70
	uuidVariantRfc = 0x80
)

// NewUUIDv7 собирает UUID версии 7: 48 бит unix-milliseconds + случайный хвост.
//
//nolint:mnd // маски 0x0F/0x3F — часть раскладки UUIDv7
func NewUUIDv7() string {
	var b [16]byte

	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> shift6)
	b[1] = byte(ms >> shift5)
	b[2] = byte(ms >> shift4)
	b[3] = byte(ms >> shift3)
	b[4] = byte(ms >> shift2)
	b[5] = byte(ms)

	_, _ = rand.Read(b[6:])

	b[6] = (b[6] & 0x0F) | uuidVersion7   // version 7
	b[8] = (b[8] & 0x3F) | uuidVariantRfc // variant RFC 4122

	var dst [36]byte
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])

	return string(dst[:])
}
