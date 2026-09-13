package messages

import (
	"testing"
	"time"
)

func TestAppleTime(t *testing.T) {
	for _, test := range []struct {
		value int64
		want  time.Time
	}{
		{0, time.Time{}},
		{-1, time.Time{}},
		{500_000_000, time.Date(2016, 11, 5, 0, 53, 20, 0, time.UTC)},
		{500_000_000_123_456_789, time.Date(2016, 11, 5, 0, 53, 20, 123456789, time.UTC)},
	} {
		if got := AppleTime(test.value); got != test.want {
			t.Errorf("AppleTime(%d) = %v, want %v", test.value, got, test.want)
		}
	}
}
