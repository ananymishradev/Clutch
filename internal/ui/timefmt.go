package ui

import (
	"fmt"
	"time"
)

func Relative(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%02d:%02d:%02d", int(d/time.Hour), int(d/time.Minute)%60, int(d/time.Second)%60)
}
