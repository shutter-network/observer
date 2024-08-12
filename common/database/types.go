package database

import (
	"math"

	"github.com/jackc/pgx/v5/pgtype"
)

// This utility function convert uint64 to pgtype.Int8.
// In the case if data overflows it returns Valid false
// which means its null in postgres types
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
