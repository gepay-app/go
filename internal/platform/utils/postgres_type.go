package utils

import (
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

func ToPgText(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  true,
	}
}

func ToPgTextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{
			Valid: false}
	}

	return pgtype.Text{
		String: *s,
		Valid:  *s != "",
	}
}

// bool
func ToPgBool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

func ToPgBoolPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}

	return pgtype.Bool{Bool: *b, Valid: true}
}

// SMALLINT / int16

func ToPgInt2(n int16) pgtype.Int2 {
	return pgtype.Int2{Int16: n, Valid: true}
}

func ToPgInt2Ptr(n *int16) pgtype.Int2 {
	if n == nil {
		return pgtype.Int2{Valid: false}
	}

	return pgtype.Int2{Int16: *n, Valid: true}
}

// INTEGER / int32

func ToPgInt4(n int32) pgtype.Int4 {
	return pgtype.Int4{Int32: n, Valid: true}
}

func ToPgInt4Ptr(n *int32) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{Valid: false}
	}

	return pgtype.Int4{Int32: *n, Valid: true}
}

// BIGINT / int64

func ToPgInt8(n int64) pgtype.Int8 {
	return pgtype.Int8{Int64: n, Valid: true}
}

func ToPgInt8Ptr(n *int64) pgtype.Int8 {
	if n == nil {
		return pgtype.Int8{Valid: false}
	}

	return pgtype.Int8{Int64: *n, Valid: true}
}

// REAL / float32

func ToPgFloat4(n float32) pgtype.Float4 {
	return pgtype.Float4{Float32: n, Valid: true}
}

func ToPgFloat4Ptr(n *float32) pgtype.Float4 {
	if n == nil {
		return pgtype.Float4{Valid: false}
	}

	return pgtype.Float4{Float32: *n, Valid: true}
}

// DOUBLE PRECISION / float64

func ToPgFloat8(n float64) pgtype.Float8 {
	return pgtype.Float8{Float64: n, Valid: true}
}

func ToPgFloat8Ptr(n *float64) pgtype.Float8 {
	if n == nil {
		return pgtype.Float8{Valid: false}
	}

	return pgtype.Float8{Float64: *n, Valid: true}
}

// time
func ToPgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}
}

func ToPgTimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}

	return pgtype.Timestamptz{
		Time:  *t,
		Valid: true,
	}
}
