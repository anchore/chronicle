package internal

import "time"

func FormatDateTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05 MST")
}
