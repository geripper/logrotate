package ticker

import (
	"time"
)

func CalRotateTimeDuration(now time.Time) time.Duration {
	nextRotateTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, 1).Unix() - now.Unix()
	return time.Duration(nextRotateTime)
}
