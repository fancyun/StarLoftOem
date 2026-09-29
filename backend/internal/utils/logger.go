package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// 日志类型（文件按类型分开，便于查询）
const (
	LogAccess   = "access"   // HTTP 访问日志
	LogBusiness = "business" // 业务/系统日志（标准 log 输出）
	LogError    = "error"    // 错误日志
	LogAdmin    = "admin"    // 管理后台操作日志
	LogSyscall  = "syscall"  // 对第三方服务的调用/回调日志
)

// 全局日志器（在 InitLoggers 中初始化）
var (
	AccessLogger  *log.Logger // 访问日志：access.log
	ErrorLogger   *log.Logger // 错误日志：error.log（同时输出到控制台 stderr）
	AdminLogger   *log.Logger // 管理操作日志：admin.log
	SysCallLogger *log.Logger // 第三方调用/回调日志：syscall.log
)

// fileWriter 追加写入器：每个类别固定写一个文件，只追加、不轮转、不删除、不覆盖。
// 日志文件随运行时间持续增长，历史全量保留（排查/对账/回填依赖完整历史）。
type fileWriter struct {
	mu       sync.Mutex
	dir      string
	filename string

	file *os.File
}

// newFileWriter 创建追加写入器（目录不存在时创建）
func newFileWriter(dir, filename string) (*fileWriter, error) {
	w := &fileWriter{dir: dir, filename: filename}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

// open 打开（或新建）日志文件（O_APPEND：已有内容只追加，不截断）
func (w *fileWriter) open() error {
	path := filepath.Join(w.dir, w.filename)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.file = file
	return nil
}

// Write 实现 io.Writer，直接追加写入（不轮转、不删除）
func (w *fileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

// InitLoggers 初始化文件日志（写入 logDir，类型分开存放）
// 将标准 log 包输出重定向到 business.log，同时创建 access/error/admin 日志器。
// 通过 LOG_DIR 环境变量可覆盖目录（默认 /app/logs）。
func InitLoggers(logDir string) error {
	if logDir == "" {
		logDir = "/app/logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 业务/系统日志（标准 log 输出重定向）：写文件的同时输出到控制台 stdout
	businessWriter, err := newFileWriter(logDir, LogBusiness+".log")
	if err != nil {
		return fmt.Errorf("初始化业务日志失败: %w", err)
	}
	log.SetOutput(io.MultiWriter(businessWriter, os.Stdout))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 访问日志：写文件的同时输出到控制台
	accessWriter, err := newFileWriter(logDir, LogAccess+".log")
	if err != nil {
		return fmt.Errorf("初始化访问日志失败: %w", err)
	}
	AccessLogger = log.New(io.MultiWriter(accessWriter, os.Stdout), "", log.LstdFlags)

	// 错误日志：写 error.log，同时输出到控制台 stderr
	errorWriter, err := newFileWriter(logDir, LogError+".log")
	if err != nil {
		return fmt.Errorf("初始化错误日志失败: %w", err)
	}
	ErrorLogger = log.New(io.MultiWriter(errorWriter, os.Stderr), "ERROR: ", log.LstdFlags)

	// 管理操作日志：写文件的同时输出到控制台
	adminWriter, err := newFileWriter(logDir, LogAdmin+".log")
	if err != nil {
		return fmt.Errorf("初始化管理日志失败: %w", err)
	}
	AdminLogger = log.New(io.MultiWriter(adminWriter, os.Stdout), "", log.LstdFlags)

	// 对第三方服务的调用/回调日志：写文件的同时输出到控制台
	syscallWriter, err := newFileWriter(logDir, LogSyscall+".log")
	if err != nil {
		return fmt.Errorf("初始化第三方调用日志失败: %w", err)
	}
	SysCallLogger = log.New(io.MultiWriter(syscallWriter, os.Stdout), "", log.LstdFlags)

	log.Printf("文件日志已初始化，目录: %s（business/access/admin/syscall/error 均同时写文件与控制台）", logDir)
	return nil
}
