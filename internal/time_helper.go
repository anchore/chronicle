package internal

import "time"

func FormatDateTime(t time.Time) string {
	return t.Format("YYYY-MM-DD 15:04")
}
