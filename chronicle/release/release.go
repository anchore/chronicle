package release

import (
	"time"
)

type Release struct {
	Version string
	Date    time.Time
}
