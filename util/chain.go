package util

import (
	"time"
)

const (
	// DefaultInitHeight 0 高度
	DefaultInitHeight int64 = 1598306400
)

func TimeToHeight(timestamp int64) int64 {
	return (timestamp - DefaultInitHeight) / 30
}

func CurrentHeight() int64 {
	currentTime := time.Now().Unix()
	return TimeToHeight(currentTime)
}
