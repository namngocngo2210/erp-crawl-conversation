package dto

import (
	"fmt"
	"strings"
	"time"
)

// PancakeTime parses Pancake timestamps that have no timezone suffix,
// e.g. "2026-06-14T02:00:49.000000" or "2026-06-14T02:00:56". Treated as UTC.
type PancakeTime struct{ time.Time }

var pancakeLayouts = []string{
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
}

func (t *PancakeTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	for _, l := range pancakeLayouts {
		if tm, err := time.Parse(l, s); err == nil {
			t.Time = tm.UTC()
			return nil
		}
	}
	return fmt.Errorf("dto: cannot parse pancake time %q", s)
}
