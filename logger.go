package log

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)
const TimeFORMAT = "2006-01-02 15:04:05"

type logEntry struct {
	level  LogLevel
	msg    string
	logger *Logger
}

type Logger struct {
	level         LogLevel
	timeFormat    string
	logChan       chan *logEntry
	wg            sync.WaitGroup
	file          *os.File
	filename      string
	maxFileSize   int64
	maxFileBackup int
}

var instance *Logger
var once sync.Once

func NewLogger(level LogLevel, filename string, maxFileSizeMB int64, maxFileBackupMB int) (*Logger, error) {
	var err error
	once.Do(func() {
		instance = &Logger{
			level:         level,
			timeFormat:    TimeFORMAT,
			logChan:       make(chan *logEntry),
			filename:      filename,
			maxFileSize:   maxFileSizeMB * 1024 * 1024,
			maxFileBackup: maxFileBackupMB * 1024 * 1024,
		}
		if err = instance.initFile(); err != nil {
			return
		}
		go instance.writeLogEntries()
	})
	return instance, err
}
func Debug(format string, args ...interface{}) {
	instance.debugf(format, args)
}
func Info(format string, args ...interface{}) {
	instance.infof(format, args)
}
func Warning(format string, args ...interface{}) {
	instance.infof(format, args)
}
func Error(format string, args ...interface{}) {
	instance.errorf(format, args)
}
func (logger *Logger) logf(level LogLevel, format string, args ...interface{}) {
	if logger.level > level {
		return
	}

	entry := &logEntry{
		level:  level,
		msg:    fmt.Sprintf(format, args...),
		logger: logger,
	}

	logger.logChan <- entry
}

func (logger *Logger) debugf(format string, args ...interface{}) {
	logger.logf(DEBUG, format, args...)
}

func (logger *Logger) infof(format string, args ...interface{}) {
	logger.logf(INFO, format, args...)
}

func (logger *Logger) warningf(format string, args ...interface{}) {
	logger.logf(WARNING, format, args...)
}

func (logger *Logger) errorf(format string, args ...interface{}) {
	logger.logf(ERROR, format, args...)
}

func (logger *Logger) writeLogEntries() {
	for entry := range logger.logChan {
		logger.wg.Add(1)
		go func(entry *logEntry) {
			defer logger.wg.Done()
			logger.writeLogEntry(entry)
		}(entry)
	}
	logger.wg.Wait()
}

func (logger *Logger) writeLogEntry(entry *logEntry) {
	msg := entry.msg
	timestamp := time.Now().Format(logger.timeFormat)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, levelToString(entry.level), msg)

	if _, err := logger.file.Write([]byte(logLine)); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing log file: %v", err)
	}
	logger.checkFileRotation()
}

func (logger *Logger) checkFileRotation() {
	fileInfo, err := logger.file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking log file size: %v", err)
		return
	}

	if fileInfo.Size() >= logger.maxFileSize {
		if err := logger.rotateLogFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Error rotating log file: %v", err)
		}
	}
}
func (logger *Logger) rotateLogFile() error {
	logger.file.Close()

	// Rename backup files
	for i := logger.maxFileBackup - 1; i >= 0; i-- {
		oldName := fmt.Sprintf("%s.%d", logger.filename, i)
		newName := fmt.Sprintf("%s.%d", logger.filename, i+1)
		os.Rename(oldName, newName)
	}

	// Rename current file
	os.Rename(logger.filename, fmt.Sprintf("%s.1", logger.filename))

	return logger.initFile()
}
func (logger *Logger) initFile() error {
	file, err := os.OpenFile(logger.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		return fmt.Errorf("error opening log file: %v", err)
	}
	logger.file = file

	return nil
}
func levelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
