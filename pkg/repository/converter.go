package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPgtype converts uuid.UUID to pgtype.UUID
func UUIDToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// PgtypeToUUID converts pgtype.UUID to uuid.UUID
func PgtypeToUUID(id pgtype.UUID) uuid.UUID {
	return id.Bytes
}

// StringPtrToPgtext converts *string to pgtype.Text
func StringPtrToPgtext(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// PgtextToStringPtr converts pgtype.Text to *string
func PgtextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// TimestampToPgtype converts time.Time to pgtype.Timestamp
func TimestampToPgtype(t time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{Time: t, Valid: true}
}

// PgtypeToTime converts pgtype.Timestamp to time.Time
func PgtypeToTime(t pgtype.Timestamp) time.Time {
	return t.Time
}

// PgtypeToTimePtr converts pgtype.Timestamp to *time.Time
func PgtypeToTimePtr(t pgtype.Timestamp) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// Int32ToPgtype converts int32 to pgtype.Int4
func Int32ToPgtype(i int32) pgtype.Int4 {
	return pgtype.Int4{Int32: i, Valid: true}
}

// Int32PtrToPgtype converts *int32 to pgtype.Int4
func Int32PtrToPgtype(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}

// IntPtrToPgtype converts *int to pgtype.Int4
func IntPtrToPgtype(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// IntToPgtype converts int to pgtype.Int4
func IntToPgtype(i int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(i), Valid: true}
}

// PgtypeToInt32Ptr converts pgtype.Int4 to *int32
func PgtypeToInt32Ptr(i pgtype.Int4) *int32 {
	if !i.Valid {
		return nil
	}
	return &i.Int32
}

// PgtypeToIntPtr converts pgtype.Int4 to *int
func PgtypeToIntPtr(i pgtype.Int4) *int {
	if !i.Valid {
		return nil
	}
	val := int(i.Int32)
	return &val
}

// PgtypeToInt converts pgtype.Int4 to int (falls back to 0 if invalid)
func PgtypeToInt(i pgtype.Int4) int {
	if !i.Valid {
		return 0
	}
	return int(i.Int32)
}
