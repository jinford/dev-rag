package postgres

import (
	"encoding/json"
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

// UUIDPtrToPgtype converts *uuid.UUID to pgtype.UUID
func UUIDPtrToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

// PgtypeToUUIDPtr converts pgtype.UUID to *uuid.UUID
func PgtypeToUUIDPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	uid := uuid.UUID(id.Bytes)
	return &uid
}

// StringPtrToPgtext converts *string to pgtype.Text
func StringPtrToPgtext(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// StringToNullableText converts string to pgtype.Text (nullable)
func StringToNullableText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// PgtextToStringPtr converts pgtype.Text to *string
func PgtextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// TimeToPgtype converts time.Time to pgtype.Timestamp
func TimeToPgtype(t time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{Time: t, Valid: true}
}

// PgtypeToTime converts pgtype.Timestamp to time.Time
func PgtypeToTime(t pgtype.Timestamp) time.Time {
	return t.Time
}

// TimePtrToPgtype converts *time.Time to pgtype.Timestamp
func TimePtrToPgtype(t *time.Time) pgtype.Timestamp {
	if t == nil {
		return pgtype.Timestamp{}
	}
	return pgtype.Timestamp{Time: *t, Valid: true}
}

// PgtypeToTimePtr converts pgtype.Timestamp to *time.Time
func PgtypeToTimePtr(t pgtype.Timestamp) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// IntToPgtype converts int to pgtype.Int4
func IntToPgtype(i int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(i), Valid: true}
}

// PgtypeToInt converts pgtype.Int4 to int
func PgtypeToInt(i pgtype.Int4) int {
	if !i.Valid {
		return 0
	}
	return int(i.Int32)
}

// IntPtrToPgInt4 converts *int to pgtype.Int4
func IntPtrToPgInt4(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// PgtypeToIntPtr converts pgtype.Int4 to *int
func PgtypeToIntPtr(i pgtype.Int4) *int {
	if !i.Valid {
		return nil
	}
	val := int(i.Int32)
	return &val
}

// Float64PtrToPgNumeric converts *float64 to pgtype.Numeric
func Float64PtrToPgNumeric(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var num pgtype.Numeric
	_ = num.Scan(*f)
	return num
}

// Float64ToNullableNumeric converts float64 to pgtype.Numeric
func Float64ToNullableNumeric(f float64) pgtype.Numeric {
	var num pgtype.Numeric
	_ = num.Scan(f)
	return num
}

// PgtypeToFloat64Ptr converts pgtype.Numeric to *float64
func PgtypeToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, _ := n.Float64Value()
	val := f.Float64
	return &val
}

// PgnumericToFloat64 converts pgtype.Numeric to float64
func PgnumericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0.0
	}
	f, _ := n.Float64Value()
	return f.Float64
}

// JSONBFromStringSlice converts []string to []byte (JSONB)
func JSONBFromStringSlice(s []string) []byte {
	if s == nil {
		return nil
	}
	b, _ := json.Marshal(s)
	return b
}

// StringSliceFromJSONB converts []byte (JSONB) to []string
func StringSliceFromJSONB(b []byte) []string {
	if b == nil {
		return nil
	}
	var s []string
	_ = json.Unmarshal(b, &s)
	return s
}
