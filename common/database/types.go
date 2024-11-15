package database

import (
	"math"

	"github.com/jackc/pgx/v5/pgtype"
)

// Uint64ToPgTypeInt8 converts a uint64 to a pgtype.Int8.
// If the input overflows, it returns a SQL `NULL` value.
func Uint64ToPgTypeInt8(data uint64) pgtype.Int8 {
	if data > math.MaxInt64 {
		return pgtype.Int8{Int64: 0, Valid: false}
	}
	return pgtype.Int8{Int64: int64(data), Valid: true}
}

// This utility function convert bool to pgtype.Bool.
func BoolToPgTypeBool(data bool) pgtype.Bool {
	return pgtype.Bool{Bool: data, Valid: true}
}

// Int64ToPgTypeInt8 converts a int64 to a pgtype.Int8.
func Int64ToPgTypeInt8(data int64) pgtype.Int8 {
	return pgtype.Int8{Int64: data, Valid: true}
}
