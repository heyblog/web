package mail

import (
	"fmt"
	"time"
)

func FormatValidity(validity time.Duration) string {
	switch {
	case validity%time.Hour == 0:
		return fmt.Sprintf("%d 小时", validity/time.Hour)
	case validity%time.Minute == 0:
		return fmt.Sprintf("%d 分钟", validity/time.Minute)
	default:
		return fmt.Sprintf("%d 秒", validity/time.Second)
	}
}
