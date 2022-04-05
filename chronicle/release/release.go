package release

import (
	"time"
)

// Release represents a version of software at a point in time.
type Release struct {
	Version string
	Date    time.Time
}
