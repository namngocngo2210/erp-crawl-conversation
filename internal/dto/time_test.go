package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPancakeTime_NoTZMicros(t *testing.T) {
	var pt PancakeTime
	require.NoError(t, pt.UnmarshalJSON([]byte(`"2026-06-14T02:00:49.000000"`)))
	require.Equal(t, 2026, pt.Year())
	require.Equal(t, time.June, pt.Month())
	require.Equal(t, time.UTC, pt.Location())
}

func TestPancakeTime_Seconds(t *testing.T) {
	var pt PancakeTime
	require.NoError(t, pt.UnmarshalJSON([]byte(`"2026-06-14T02:00:56"`)))
	require.Equal(t, 56, pt.Second())
}

func TestPancakeTime_Null(t *testing.T) {
	var pt PancakeTime
	require.NoError(t, pt.UnmarshalJSON([]byte(`null`)))
	require.True(t, pt.IsZero())
}

func TestPancakeTime_Invalid(t *testing.T) {
	var pt PancakeTime
	require.Error(t, pt.UnmarshalJSON([]byte(`"not-a-time"`)))
}
