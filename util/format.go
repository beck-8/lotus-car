package util

import "fmt"

const (
	_          = iota // ignore first value by assigning to blank identifier
	KB float64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB
)

// FormatSize converts bytes to human readable string. Takes a
// size in bytes and returns a human-readable string.
func FormatSize(bytes int64) string {
	unit := ""
	size := float64(bytes)

	switch {
	case size >= PB:
		unit = "PB"
		size = size / PB
	case size >= TB:
		unit = "TB"
		size = size / TB
	case size >= GB:
		unit = "GB"
		size = size / GB
	case size >= MB:
		unit = "MB"
		size = size / MB
	case size >= KB:
		unit = "KB"
		size = size / KB
	default:
		unit = "B"
	}

	if unit == "B" {
		return fmt.Sprintf("%.0f %s", size, unit)
	}
	return fmt.Sprintf("%.2f %s", size, unit)
}
