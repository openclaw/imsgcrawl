package cli

import (
	"testing"
	"time"
)

func TestFormatAppleDateSupportsLegacySeconds(t *testing.T) {
	want := time.Date(2016, 11, 5, 0, 53, 20, 0, time.UTC).Local().Format("2006-01-02 15:04")
	for _, value := range []int64{500_000_000, 500_000_000_000_000_000} {
		if got := formatAppleDate(value); got != want {
			t.Errorf("formatAppleDate(%d) = %q, want %q", value, got, want)
		}
	}
	for _, value := range []int64{0, -1} {
		if got := formatAppleDate(value); got != "-" {
			t.Errorf("formatAppleDate(%d) = %q, want -", value, got)
		}
	}
}
