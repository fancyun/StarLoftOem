package router

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/utils"
)

// metricsHandler 输出 Prometheus 文本格式的运行指标（用于监控采集 Prometheus /metrics）
func metricsHandler(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.Writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintf(c.Writer, "# HELP go_goroutines Number of goroutines that currently exist.\n")
	fmt.Fprintf(c.Writer, "# TYPE go_goroutines gauge\n")
	fmt.Fprintf(c.Writer, "go_goroutines %d\n", runtime.NumGoroutine())

	fmt.Fprintf(c.Writer, "# HELP go_memstats_alloc_bytes_bytes Number of bytes allocated and still in use.\n")
	fmt.Fprintf(c.Writer, "# TYPE go_memstats_alloc_bytes_bytes gauge\n")
	fmt.Fprintf(c.Writer, "go_memstats_alloc_bytes_bytes %d\n", m.Alloc)

	fmt.Fprintf(c.Writer, "# HELP go_memstats_sys_bytes Number of bytes obtained from system.\n")
	fmt.Fprintf(c.Writer, "# TYPE go_memstats_sys_bytes gauge\n")
	fmt.Fprintf(c.Writer, "go_memstats_sys_bytes %d\n", m.Sys)

	fmt.Fprintf(c.Writer, "# HELP process_uptime_seconds Process uptime in seconds.\n")
	fmt.Fprintf(c.Writer, "# TYPE process_uptime_seconds gauge\n")
	fmt.Fprintf(c.Writer, "process_uptime_seconds %.0f\n", time.Since(utils.ProcessStartTime).Seconds())
}
