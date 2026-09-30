package utils

import (
	"bytes"
	"io"
	"os"
	"strings"
)

const (
	// tailReadMaxBytes 单次尾部读取窗口上限（日志文件只追加不轮转，可能持续增长，故按窗口读尾部）
	tailReadMaxBytes = 2 << 20 // 2 MiB
	// maxLogLineRunes 单行返回长度上限（第三方回调报文可能很长，超出部分截断）
	maxLogLineRunes = 4000
)

// TailFile 读取文件尾部窗口并按行返回：最多读取 maxBytes 字节，最多返回 lines 行。
// 返回 (行列表, 是否只读取了尾部窗口, error)。文件不存在时返回 os.ErrNotExist。
// 实现要点：一律 f.ReadAt 从尾部按窗口读取（绝不整文件载入）；截断的首行丢弃并对齐 UTF-8 起点；
// 每行做 UTF-8 兜底与长度截断，避免并发写入踩到半个多字节字符导致 JSON 非法。
func TailFile(path string, lines int, maxBytes int64) ([]string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	size := st.Size()
	if size <= 0 {
		return []string{}, false, nil
	}
	if maxBytes <= 0 {
		maxBytes = tailReadMaxBytes
	}

	readSize := size
	truncated := false
	if readSize > maxBytes {
		readSize = maxBytes
		truncated = true
	}
	offset := size - readSize
	buf := make([]byte, readSize)
	if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
		return nil, false, err
	}

	if offset > 0 {
		// 丢弃被截断的首行
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		} else {
			buf = nil
		}
		// 对齐 UTF-8 起始边界（跳过续字节 0b10xxxxxx）
		for i := 0; i < len(buf) && i < 3; i++ {
			if buf[i]&0xC0 != 0x80 {
				buf = buf[i:]
				break
			}
		}
	}

	text := strings.ToValidUTF8(string(buf), "\uFFFD")
	out := make([]string, 0, lines)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if runes := []rune(line); len(runes) > maxLogLineRunes {
			line = string(runes[:maxLogLineRunes]) + "…"
		}
		out = append(out, line)
	}
	if lines > 0 && len(out) > lines {
		out = out[len(out)-lines:]
	}
	return out, truncated, nil
}