package recency

import (
	"os"
	"time"
)

func statMtime(path string) (time.Time, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}
