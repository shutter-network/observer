package database

import "github.com/jackc/pgx/v5/pgtype"

func Uint8ToPgTypeInt8(data int64) pgtype.Int8 {
	return pgtype.Int8{Int64: data, Valid: true}
}

func Uint64ToPgTypeInt8(data uint64) pgtype.Int8 {
	return pgtype.Int8{Int64: int64(data), Valid: true}
}

func BoolToPgTypeBool(data bool) pgtype.Bool {
	return pgtype.Bool{Bool: data, Valid: true}
}
