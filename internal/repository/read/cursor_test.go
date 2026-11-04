package read

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	in := Cursor{Time: time.Unix(0, 1717000000123456789).UTC(), ID: 42}
	s := EncodeCursor(in)
	out, ok := DecodeCursor(s)
	if !ok {
		t.Fatal("decode failed")
	}
	if !out.Time.Equal(in.Time) || out.ID != in.ID {
		t.Fatalf("round trip mismatch: %+v vs %+v", out, in)
	}
}

func TestDecodeEmpty(t *testing.T) {
	if _, ok := DecodeCursor(""); ok {
		t.Fatal("empty should not decode")
	}
	if _, ok := DecodeCursor("not-base64!!"); ok {
		t.Fatal("garbage should not decode")
	}
}
