package messages

import "time"

// AppleTime decodes legacy seconds or modern nanoseconds since January 1, 2001.
func AppleTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	const epoch = 978307200
	if value < 1_000_000_000_000 {
		return time.Unix(epoch+value, 0).UTC()
	}
	return time.Unix(epoch, value).UTC()
}
