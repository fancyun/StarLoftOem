package utils

import "time"

// ProcessStartTime 进程启动时间（供 /metrics 与后台「系统监控」共用，避免多处各自记时）
var ProcessStartTime = time.Now()