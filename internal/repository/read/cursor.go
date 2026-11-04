package read

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

type Cursor struct {
	Time time.Time
	ID   int64
}

func EncodeCursor(c Cursor) string {
	raw := strconv.FormatInt(c.Time.UTC().UnixNano(), 10) + "," + strconv.FormatInt(c.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (Cursor, bool) {
	if s == "" {
		return Cursor{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, false
	}
	parts := strings.SplitN(string(b), ",", 2)
	if len(parts) != 2 {
		return Cursor{}, false
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Cursor{}, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Cursor{}, false
	}
	return Cursor{Time: time.Unix(0, ns).UTC(), ID: id}, true
}
