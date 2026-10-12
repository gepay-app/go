package utils

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// Pagination keyset (cursor), bukan OFFSET.
//
// Cursor cukup memakai `id` — UUIDv7 dan `GENERATED ALWAYS AS IDENTITY` sama-sama
// punya urutan total, jadi tidak perlu (created_at, id). Ini juga bebas tipe:
// nilai id disandikan sebagai string, pemanggil yang punya tipenya (uuid/bigint).
//
// Pola pakai di handler:
//
//	cursorID := uuid.Max() // halaman pertama: id terbesar, jadi semua baris lolos
//	if raw := c.QueryParam("cursor"); raw != "" {
//		s, _ := utils.DecodeCursor(raw)
//		cursorID, _ = uuid.Parse(s)
//	}
//	limit := utils.ParseLimit(c.QueryParam("limit"), 20, 100)
//	rows, _ := repo.ListX(ctx, ListXParams{ID: cursorID, Limit: int32(limit + 1)})
//	hasMore := len(rows) > limit
//	if hasMore { rows = rows[:limit] }
//	// next_cursor = utils.EncodeCursor(rows[len(rows)-1].ID.String())

// EncodeCursor menyandikan id jadi cursor opaque (base64url) untuk dikirim ke client.
func EncodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

// DecodeCursor mengurai cursor. String kosong = halaman pertama ("").
func DecodeCursor(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return "", errors.New("invalid cursor")
	}
	return string(b), nil
}

// ParseLimit membaca query "limit". Nilai kosong/invalid/<=0 -> def; di atas max
// -> max. Batasi selalu supaya client tidak bisa meminta halaman raksasa.
func ParseLimit(raw string, def, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
