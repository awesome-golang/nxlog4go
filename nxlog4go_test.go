// Copyright (C) 2010, Kyle Lemons <kyle@kylelemons.net>.  All rights reserved.

package nxlog4go

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"
	"crypto/md5"
	"encoding/hex"
)

const testLogFile = "_logtest.log"
const benchLogFile = "_benchlog.log"

var now time.Time = time.Unix(0, 1234567890123456789).In(time.UTC)

func newLogRecord(lvl Level, prefix, src string, msg string) *LogRecord {
	return &LogRecord{
		Level:   lvl,
		Source:  src,
		Prefix:  prefix,
		Created: now,
		Message: msg,
	}
}

func TestELog(t *testing.T) {
	fmt.Printf("Testing %s\n", NXLOG4GO_VERSION)
	lr := newLogRecord(CRITICAL, "prefix", "source", "message")
	if lr.Level != CRITICAL {
		t.Errorf("Incorrect level: %d should be %d", lr.Level, CRITICAL)
	}
	if lr.Prefix != "prefix" {
		t.Errorf("Incorrect prefix: %s should be %s", lr.Prefix, "prefix")
	}
	if lr.Source != "source" {
		t.Errorf("Incorrect source: %s should be %s", lr.Source, "source")
	}
	if lr.Message != "message" {
		t.Errorf("Incorrect message: %s should be %s", lr.Source, "message")
	}
}

var patternTests = []struct {
	Test    string
	Record  *LogRecord
	Patterns map[string]string
}{
	{
		Test: "Standard formats",
		Record: &LogRecord{
			Level:   ERROR,
			Source:  "source",
			Message: "message",
			Created: now,
		},
		Patterns: map[string]string{
			// TODO(kevlar): How can I do this so it'll work outside of PST?
			PATTERN_DEFAULT: "[2009/02/13 23:31:30 CST] [EROR] (source) message\n",
			PATTERN_SHORT:   "[23:31 13/02/09] [EROR] message\n",
			PATTERN_ABBREV:  "[EROR] message\n",
		},
	},
}

func TestPatternLayout(t *testing.T) {
	for _, test := range patternTests {
		name := test.Test
		for pattern, want := range test.Patterns {
			layout := NewPatternLayout(pattern)
			if got := string(layout.Format(test.Record)); got != want {
				t.Errorf("%s - %s:", name, pattern)
				t.Errorf("   got %q", got)
				t.Errorf("  want %q", want)
			}
		}
	}
}

var logRecordWriteTests = []struct {
	Test    string
	Record  *LogRecord
	Console string
}{
	{
		Test: "Normal message",
		Record: &LogRecord{
			Level:   CRITICAL,
			Source:  "source",
			Message: "message",
			Created: now,
		},
		Console: "[2009/02/13 23:31:30 CST] [CRIT] (source) message",
	},
}

func TestConsoleWriter(t *testing.T) {
	r, w := io.Pipe()

	buf := make([]byte, 1024)

	layout := NewPatternLayout(PATTERN_DEFAULT)
	for _, test := range logRecordWriteTests {
		name := test.Test

		// Pipe write and read must be in diff routines otherwise cause dead lock
		go w.Write(layout.Format(test.Record))

		n, _ := r.Read(buf)
		if got, want := string(buf[:n]), test.Console; got != (want+"\n") {
			t.Errorf("%s:  got %q", name, got)
			t.Errorf("%s: want %q", name, want)
		}
	}
}

func TestFileWriter(t *testing.T) {
	w := NewFileBufWriter(testLogFile).SetFlush(0)

	defer os.Remove(testLogFile)

	layout := NewPatternLayout(PATTERN_DEFAULT)
	w.Write(layout.Format(newLogRecord(CRITICAL, "prefix", "source", "message")))
	w.Close()

	runtime.Gosched()

	if contents, err := ioutil.ReadFile(testLogFile); err != nil {
		t.Errorf("read(%q): %s", testLogFile, err)
	} else if len(contents) != 50 {
		t.Errorf("malformed filelog: %q (%d bytes)", string(contents), len(contents))
	}
}

func TestRotateFileWriter(t *testing.T) {
	w := NewRotateFileWriter(testLogFile).SetFlush(0)

	defer os.Remove(testLogFile)

	layout := NewPatternLayout(PATTERN_DEFAULT)
	w.Write(layout.Format(newLogRecord(CRITICAL, "prefix", "source", "message")))
	w.Close()

	runtime.Gosched()

	if contents, err := ioutil.ReadFile(testLogFile); err != nil {
		t.Errorf("read(%q): %s", testLogFile, err)
	} else if len(contents) != 50 {
		t.Errorf("malformed filelog: %q (%d bytes)", string(contents), len(contents))
	}
}

func TestLogger(t *testing.T) {
	l := New(WARNING)
	if l == nil {
		t.Fatalf("New should never return nil")
	}
	if l.level != WARNING {
		t.Fatalf("New produced invalid logger (incorrect level)")
	}

	//func (l *Logger) Warn(format string, args ...interface{}) error {}
	if err := l.Warn("%s %d %#v", "Warning:", 1, []int{}); err.Error() != "Warning: 1 []int{}" {
		t.Errorf("Warn returned invalid error: %s", err)
	}

	//func (l *Logger) Error(format string, args ...interface{}) error {}
	if err := l.Error("%s %d %#v", "Error:", 10, []string{}); err.Error() != "Error: 10 []string{}" {
		t.Errorf("Error returned invalid error: %s", err)
	}

	//func (l *Logger) Critical(format string, args ...interface{}) error {}
	if err := l.Critical("%s %d %#v", "Critical:", 100, []int64{}); err.Error() != "Critical: 100 []int64{}" {
		t.Errorf("Critical returned invalid error: %s", err)
	}
	// Already tested or basically untestable
	//func (l *Logger) Log(level int, source, message string) {}
	//func (l *Logger) Logf(level int, format string, args ...interface{}) {}
	//func (l *Logger) intLogf(level int, format string, args ...interface{}) string {}
	//func (l *Logger) Finest(format string, args ...interface{}) {}
	//func (l *Logger) Fine(format string, args ...interface{}) {}
	//func (l *Logger) Debug(format string, args ...interface{}) {}
	//func (l *Logger) Trace(format string, args ...interface{}) {}
	//func (l *Logger) Info(format string, args ...interface{}) {}
}

func TestLogOutput(t *testing.T) {
	const (
		expected = "fdf3e51e444da56b4cb400f30bc47424"
	)

	fbw := NewFileBufWriter(testLogFile).SetFlush(0)
	ww := io.MultiWriter(os.Stderr, fbw)
	l := New(FINEST).SetOutput(ww).SetPattern("[%L] %M")

	defer os.Remove(testLogFile)

	// Send some log messages
	l.intLog(CRITICAL, fmt.Sprintf("This message is level %d", int(CRITICAL)))
	l.intLog(ERROR, "This message is level %v", ERROR)
	l.intLog(WARNING, "This message is level %s", WARNING)
	l.intLog(INFO, func() string { return "This message is level INFO" })
	l.Trace("This message is level %d", int(TRACE))
	l.Debug("This message is level %s", DEBUG)
	l.Fine(func() string { return fmt.Sprintf("This message is level %v", FINE) })
	l.Finest("This message is level %v", FINEST)
	l.Finest(FINEST, "is also this message's level")

	fbw.Close()

	contents, err := ioutil.ReadFile(testLogFile)
	if err != nil {
		t.Fatalf("Could not read output log: %s", err)
	}

	sum := md5.New()
	sum.Write(contents)
	if sumstr := hex.EncodeToString(sum.Sum(nil)); sumstr != expected {
		t.Errorf("--- Log Contents:\n%s---", string(contents))
		t.Fatalf("Checksum does not match: %s (expecting %s)", sumstr, expected)
	}
}

func TestCountMallocs(t *testing.T) {
	const N = 1
	var m runtime.MemStats
	getMallocs := func() uint64 {
		runtime.ReadMemStats(&m)
		return m.Mallocs
	}

	// Console logger
	sl := New(INFO).SetCaller(false)

	mallocs := 0 - getMallocs()
	for i := 0; i < N; i++ {
		sl.intLog(WARNING, "This is a WARNING message")
	}
	mallocs += getMallocs()
	fmt.Printf("mallocs per sl.intLog((WARNING, \"This is a log message\"): %d\n", mallocs/N)

	// Console logger formatted
	mallocs = 0 - getMallocs()
	for i := 0; i < N; i++ {
		sl.Warn("%s is a log message with level %d", "This", WARNING)
	}
	mallocs += getMallocs()
	fmt.Printf("mallocs per sl.Warn(WARNING, \"%%s is a log message with level %%d\", \"This\", WARNING): %d\n", mallocs/N)

	// Console logger (not logged)
	sl = New(INFO)
	mallocs = 0 - getMallocs()
	for i := 0; i < N; i++ {
		sl.Debug("This is a DEBUG log message")
	}
	mallocs += getMallocs()
	fmt.Printf("mallocs per unlogged sl.Debug(WARNING, \"This is a log message\"): %d\n", mallocs/N)

	// Console logger formatted (not logged)
	mallocs = 0 - getMallocs()
	for i := 0; i < N; i++ {
		sl.Debug("%s is a log message with level %d", "This", DEBUG)
	}
	mallocs += getMallocs()
	fmt.Printf("mallocs per unlogged sl.Debug(WARNING, \"%%s is a log message with level %%d\", \"This\", WARNING): %d\n", mallocs/N)
}

func BenchmarkPatternLayout(b *testing.B) {
	const updateEvery = 1
	rec := &LogRecord{
		Level:   CRITICAL,
		Created: now,
		Prefix:  "prefix",
		Source:  "source",
		Message: "message",
	}
	lo1 := NewPatternLayout(PATTERN_DEFAULT)
	lo2 := NewPatternLayout(PATTERN_SHORT)
	for i := 0; i < b.N; i++ {
		rec.Created = rec.Created.Add(1 * time.Second / updateEvery)
		if i%2 == 0 {
			lo1.Format(rec)
		} else {
			lo2.Format(rec)
		}
	}
}

func BenchmarkConsoleWriter(b *testing.B) {
	/* This doesn't seem to work on OS X
	sink, err := os.Open(os.DevNull)
	if err != nil {
		panic(err)
	}
	if err := syscall.Dup2(int(sink.Fd()), syscall.Stdout); err != nil {
		panic(err)
	}
	*/

	sl := New(INFO).SetOutput(ioutil.Discard).SetCaller(false)
	for i := 0; i < b.N; i++ {
		sl.intLog(WARNING, "This is a log message")
	}
}

func BenchmarkConsoleUtilWriter(b *testing.B) {
	sl := New(INFO).SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		sl.Info("%s is a log message", "This")
	}
}

func BenchmarkConsoleUtilNotWriter(b *testing.B) {
	sl := New(INFO).SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		sl.Debug("%s is a log message", "This")
	}
}

func BenchmarkFileWriter(b *testing.B) {
	w := NewFileBufWriter(benchLogFile).SetFlush(0)
	defer func() {
		w.Close()
		os.Remove(benchLogFile)
	}()
	b.StopTimer()
	sl := New(INFO).SetOutput(w).SetCaller(false)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sl.intLog(WARNING, "This is a log message")
	}
	b.StopTimer()
}

func BenchmarkFileUtilWriter(b *testing.B) {
	w := NewFileBufWriter(benchLogFile).SetFlush(0)
	defer func() {
		w.Close()
		os.Remove(benchLogFile)
	}()
	defer w.Close()
	b.StopTimer()
	sl := New(INFO).SetOutput(w).SetCaller(false)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sl.Info("%s is a log message", "This")
	}
	b.StopTimer()
}

func BenchmarkFileBufWriter(b *testing.B) {
	w := NewFileBufWriter(benchLogFile)
	defer func() {
		w.Close()
		os.Remove(benchLogFile)
	}()
	b.StopTimer()
	sl := New(INFO).SetOutput(w).SetCaller(false)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sl.intLog(WARNING, "This is a log message")
	}
	b.StopTimer()
}

func BenchmarkFileBufUtilWriter(b *testing.B) {
	w := NewFileBufWriter(benchLogFile)
	defer func() {
		w.Close()
		os.Remove(benchLogFile)
	}()
	b.StopTimer()
	sl := New(INFO).SetOutput(w).SetCaller(false)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sl.Info("%s is a log message", "This")
	}
	b.StopTimer()
}

// Benchmark results (windows amd64 10g)
// BenchmarkPatternLayout-4                 1000000              1208 ns/op
// BenchmarkConsoleWriter-4                 1000000              1076 ns/op
// BenchmarkConsoleUtilWriter-4              500000              3650 ns/op
// BenchmarkConsoleUtilNotWriter-4         50000000                30.3 ns/op
// BenchmarkFileWriter-4                     200000              7085 ns/op
// BenchmarkFileUtilWriter-4                 200000              7415 ns/op
// BenchmarkFileBufWriter-4                 1000000              1473 ns/op
// BenchmarkFileBufUtilWriter-4             1000000              1543 ns/op