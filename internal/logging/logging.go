package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const retentionDays = 7

var Logger = logrus.New()

type levelFileHook struct {
	dir       string
	formatter logrus.Formatter
	mu        sync.Mutex
	files     map[string]*os.File
}

// Setup configures logrus and the stdlib slog/log bridges.
func Setup(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "log"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := cleanupOldLogs(dir, time.Now()); err != nil {
		return err
	}

	Logger.SetOutput(os.Stdout)
	Logger.SetLevel(logrus.InfoLevel)
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		DisableColors:   true,
	})
	Logger.ReplaceHooks(make(logrus.LevelHooks))
	Logger.AddHook(&levelFileHook{
		dir: dir,
		formatter: &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
			DisableColors:   true,
		},
		files: make(map[string]*os.File),
	})

	log.SetOutput(Logger.WriterLevel(logrus.InfoLevel))
	log.SetFlags(0)
	slog.SetDefault(slog.New(newSlogHandler(Logger)))
	return nil
}

func ErrorWriter() *io.PipeWriter {
	return Logger.WriterLevel(logrus.ErrorLevel)
}

func (h *levelFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *levelFileHook) Fire(entry *logrus.Entry) error {
	line, err := h.formatter.Format(entry)
	if err != nil {
		return err
	}
	date := entry.Time.Format("2006-01-02")
	level := entry.Level.String()
	path := filepath.Join(h.dir, date, level+".log")

	h.mu.Lock()
	defer h.mu.Unlock()

	f := h.files[path]
	if f == nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		h.files[path] = f
	}
	_, err = f.Write(line)
	return err
}

func cleanupOldLogs(dir string, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, -(retentionDays - 1))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		day, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

type slogHandler struct {
	logger *logrus.Logger
	attrs  []slog.Attr
	group  string
}

func newSlogHandler(logger *logrus.Logger) *slogHandler {
	return &slogHandler{logger: logger}
}

func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.logger.IsLevelEnabled(toLogrusLevel(level))
}

func (h *slogHandler) Handle(_ context.Context, record slog.Record) error {
	fields := logrus.Fields{}
	for _, attr := range h.attrs {
		addSlogAttr(fields, h.group, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		addSlogAttr(fields, h.group, attr)
		return true
	})
	entry := h.logger.WithFields(fields)
	entry.Time = record.Time
	entry.Log(toLogrusLevel(record.Level), record.Message)
	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &next
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	next := *h
	if next.group == "" {
		next.group = name
	} else {
		next.group += "." + name
	}
	return &next
}

func addSlogAttr(fields logrus.Fields, group string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	key := attr.Key
	if group != "" {
		key = group + "." + key
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			addSlogAttr(fields, key, child)
		}
		return
	}
	fields[key] = attr.Value.Any()
}

func toLogrusLevel(level slog.Level) logrus.Level {
	switch {
	case level >= slog.LevelError:
		return logrus.ErrorLevel
	case level >= slog.LevelWarn:
		return logrus.WarnLevel
	case level <= slog.LevelDebug:
		return logrus.DebugLevel
	default:
		return logrus.InfoLevel
	}
}

func PathFor(dir string, t time.Time, level logrus.Level) string {
	if strings.TrimSpace(dir) == "" {
		dir = "log"
	}
	return filepath.Join(dir, t.Format("2006-01-02"), fmt.Sprintf("%s.log", level.String()))
}
