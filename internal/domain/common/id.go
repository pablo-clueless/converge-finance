package common

import (
	"crypto/rand"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

type ID string

func NewID() ID {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return ID(id.String())
}

func NewIDFromTime(t time.Time) ID {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return ID(id.String())
}

func Parse(s string) (ID, error) {
	if s == "" {
		return "", nil
	}
	_, err := ulid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid ID format: %w", err)
	}
	return ID(s), nil
}

func MustParse(s string) ID {
	id, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

func (id ID) String() string {
	return string(id)
}

func (id ID) IsZero() bool {
	return id == ""
}

func (id ID) Time() (time.Time, error) {
	if id.IsZero() {
		return time.Time{}, fmt.Errorf("cannot get time from empty ID")
	}
	parsed, err := ulid.Parse(string(id))
	if err != nil {
		return time.Time{}, err
	}
	return ulid.Time(parsed.Time()), nil
}

func (id ID) Value() (driver.Value, error) {
	if id.IsZero() {
		return nil, nil
	}
	return string(id), nil
}

func (id *ID) Scan(value any) error {
	if value == nil {
		*id = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*id = ID(v)
	case []byte:
		*id = ID(string(v))
	default:
		return fmt.Errorf("cannot scan %T into ID", value)
	}

	return nil
}

func (id ID) MarshalJSON() ([]byte, error) {
	if id.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, id)), nil
}

func (id *ID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		*id = ""
		return nil
	}

	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}

	*id = ID(data)
	return nil
}

func (id ID) Ptr() *ID {
	if id.IsZero() {
		return nil
	}
	return &id
}

func FromPtr(id *ID) ID {
	if id == nil {
		return ""
	}
	return *id
}
