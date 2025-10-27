package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// ----------------- 全局封装 -----------------

var std *slog.Logger

// Init 初始化全局 logger。
// toFile: 是否写入文件；filePath: 文件路径（当 toFile 为 true 时生效）
func Init(toFile bool, filePath string, level slog.Level) *slog.Logger {
	var out io.Writer
	if toFile {
		// 打开文件（简单实现，不做切割）
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic(err)
		}
		out = io.MultiWriter(os.Stdout, f)
	} else {
		out = os.Stdout
	}

	opts := &slog.HandlerOptions{Level: level}
	h := NewSimpleHandler(out, opts, 3)
	std = slog.New(h)
	// 让 package-level slog.* 调用使用我们初始化的 logger（可选）
	slog.SetDefault(std)
	return std
}

// SimpleHandler 是自定义的 slog.Handler，用于输出类似 frp 风格的日志。
type SimpleHandler struct {
	out        io.Writer
	opts       *slog.HandlerOptions
	mu         sync.Mutex
	level      slog.Leveler // 可为 nil，默认 info
	callerSkip int
}

func NewSimpleHandler(out io.Writer, opts *slog.HandlerOptions, callerSkip int) *SimpleHandler {
	h := &SimpleHandler{
		out:        out,
		opts:       opts,
		callerSkip: callerSkip,
	}
	if opts != nil && opts.Level != nil {
		h.level = opts.Level
	} else {
		h.level = slog.LevelInfo
	}
	return h
}

func (h *SimpleHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	// 如果没有提供 Leveler，则默认 Info
	if h.level == nil {
		return lvl >= slog.LevelInfo
	}
	// h.level 是 Leveler，需要调用 Level() 方法
	return lvl >= h.level.Level()
}

func (h *SimpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// 不做任何预处理，返回自己（简单实现）
	return h
}

func (h *SimpleHandler) WithGroup(name string) slog.Handler {
	// 简单实现：不支持 group，直接返回自己
	return h
}

func levelShort(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "D"
	case slog.LevelInfo:
		return "I"
	case slog.LevelWarn:
		return "W"
	case slog.LevelError:
		return "E"
	default:
		// slog 支持自定义级别，为安全起见显示首字母大写
		str := strings.ToUpper(l.String())
		if len(str) > 0 {
			return str[:1]
		}
		return "?"
	}
}

func colorForLevel(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "\033[36m" // 青色
	case slog.LevelInfo:
		return "\033[32m" // 绿色
	case slog.LevelWarn:
		return "\033[33m" // 黄色
	case slog.LevelError:
		return "\033[31m" // 红色
	default:
		return "\033[0m" // 默认
	}
}

// 格式化时间为：2006-01-02 15:04:05.000 （保留毫秒）
func formatTime(t time.Time) string {
	// 使用 Local 时区格式化；如需 UTC 改为 t.UTC()
	return t.Format("2006-01-02 15:04:05.000")
}

func (h *SimpleHandler) Handle(ctx context.Context, r slog.Record) error {
	// 按建议先在内存构建好 []byte 再一次性写出，避免并发写混乱
	var b strings.Builder

	// 日志级别带颜色
	levelColor := colorForLevel(r.Level)
	b.WriteString(levelColor)
	// 时间
	if !r.Time.IsZero() {
		b.WriteString(formatTime(r.Time))
	} else {
		b.WriteString(formatTime(time.Now()))
	}
	b.WriteByte(' ')
	// 级别短字母，如 [I]
	b.WriteString("[")

	b.WriteString(levelShort(r.Level))
	b.WriteString("] ")

	// 调整 skip 数，确保拿到业务调用方
	_, file, line, ok := runtime.Caller(h.callerSkip + 2)
	if ok {
		parts := strings.Split(file, "/")
		if len(parts) > 2 {
			file = strings.Join(parts[len(parts)-2:], "/")
		}
		b.WriteString(fmt.Sprintf("[%s:%d] ", file, line))
	} else {
		b.WriteString("[unknown:0] ")
	}

	// message
	b.WriteString(r.Message)

	// attributes（按 key=value 追加在消息后）：
	// 如果没有 attrs，就不加；有 attrs 时在新的一段加空格后逐个追加 key=value
	first := true
	r.Attrs(func(a slog.Attr) bool {
		if first {
			b.WriteString(" ")
			first = false
		} else {
			b.WriteString(" ")
		}
		// 简单格式化 value
		var val string
		if a.Value.Kind() == slog.KindString {
			val = a.Value.String()
		} else {
			val = fmt.Sprint(a.Value.Any())
		}
		b.WriteString(fmt.Sprintf("%s=%s", a.Key, val))
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}

func Infof(format string, v ...any) {
	std.Info(fmt.Sprintf(format, v...))
}

func Warnf(format string, v ...any) {
	std.Warn(fmt.Sprintf(format, v...))
}

func Errorf(format string, v ...any) {
	std.Error(fmt.Sprintf(format, v...))
}

func Debugf(format string, v ...any) {
	std.Debug(fmt.Sprintf(format, v...))
}
