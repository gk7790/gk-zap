package xlog

import (
	"cmp"
	"slices"

	"github.com/gk7790/gk-zap/pkg/utils/log"
)

type LogPrefix struct {
	// Name is the name of the prefix, it won't be displayed in log but used to identify the prefix.
	Name string
	// Value is the value of the prefix, it will be displayed in log.
	Value string
	// The prefix with higher priority will be displayed first, default is 10.
	Priority int
}

type Logger struct {
	prefixes     []LogPrefix
	prefixString string
}

func New() *Logger {
	return &Logger{
		prefixes: make([]LogPrefix, 0),
	}
}

func (l *Logger) ResetPrefixes() (old []LogPrefix) {
	old = l.prefixes
	l.prefixes = make([]LogPrefix, 0)
	l.prefixString = ""
	return
}

func (l *Logger) AppendPrefix(prefix string) *Logger {
	return l.AddPrefix(LogPrefix{
		Name:     prefix,
		Value:    prefix,
		Priority: 10,
	})
}

func (l *Logger) AddPrefix(prefix LogPrefix) *Logger {
	found := false
	if prefix.Priority <= 0 {
		prefix.Priority = 10
	}
	for _, p := range l.prefixes {
		if p.Name == prefix.Name {
			found = true
			p.Value = prefix.Value
			p.Priority = prefix.Priority
		}
	}
	if !found {
		l.prefixes = append(l.prefixes, prefix)
	}
	l.renderPrefixString()
	return l
}

func (l *Logger) renderPrefixString() {
	slices.SortStableFunc(l.prefixes, func(a, b LogPrefix) int {
		return cmp.Compare(a.Priority, b.Priority)
	})
	l.prefixString = ""
	for _, v := range l.prefixes {
		l.prefixString += "[" + v.Value + "] "
	}
}

func (l *Logger) Spawn() *Logger {
	nl := New()
	nl.prefixes = append(nl.prefixes, l.prefixes...)
	nl.renderPrefixString()
	return nl
}

func (l *Logger) Errorf(format string, v ...any) {
	log.Errorf(l.prefixString+format, v...)
}

func (l *Logger) Warnf(format string, v ...any) {
	log.Warnf(l.prefixString+format, v...)
}

func (l *Logger) Infof(format string, v ...any) {
	log.Infof(l.prefixString+format, v...)
}

func (l *Logger) Debugf(format string, v ...any) {
	log.Debugf(l.prefixString+format, v...)
}
