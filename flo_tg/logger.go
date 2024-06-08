package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

var defaultLogger Logger = &dummyLogging{}

func LogErrorln(ss ...any) {
	s := fmt.Sprintln(ss...) + "\n"

	if defaultLogger == nil {
		os.Stderr.Write([]byte(s))
	} else {
		defaultLogger.Write([]byte(s))
	}
}

func LogErrorf(errf string, arg ...any) {
	s := fmt.Sprintf(errf, arg...)
	LogErrorln(s)
}

type Logger interface {
	io.Writer
	Close() error
	Message(level int32, kind string, message string, extras ...map[string]interface{}) bool
	AddRequestID(requestUid string) Logger
	CopyToStderr() Logger
	SetAsDefault() Logger
}

type dummyLogging struct {
	Logger
}

type gelfLogger struct {
	Logger
	writer                         gelf.Writer
	facility, hostname, requestUid string
	stderr                         bool
}

func NewGraylogTCPLogger(facility, graylogAddr, selfHostname string) Logger {

	gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
	if err != nil {
		log.Fatalf("gelf.NewTCPWriter: %s", err)
	}

	gelfWriter.Facility = facility

	logger := &gelfLogger{
		writer:     gelfWriter,
		facility:   facility,
		hostname:   selfHostname,
		stderr: false,
		requestUid: "",
	}

	log.Printf("Logging to stdout & graylog @%s", graylogAddr)

	return logger
}

func (dummyLogging) Close() error {
	return nil
}

func (dummyLogging) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {
	if data, err := json.MarshalIndent(extras, "", "    "); err != nil {
		log.Println("WARN log not sent", level, kind, message)
	} else {
		log.Println("WARN log not sent", level, kind, message, string(data))
	}

	return true
}

func (dummy dummyLogging) AddRequestID(string) Logger {
	return dummy
}

func (dummy dummyLogging) Write(p []byte) (int, error) {
	return 0, nil
}

func (dummy dummyLogging) SetAsDefault() Logger {
	return dummy
}

func (dummy dummyLogging) CopyToStderr() Logger {
	return dummy
}

func (logger *gelfLogger) Close() error {
	if logger.requestUid != "" {
		log.Println("WARN not setting request uid for l since it is already set", logger.requestUid)
		return nil
	}

	return logger.writer.Close()
}

func (logger *gelfLogger) AddRequestID(requestUid string) Logger {
	if logger.requestUid != "" {
		requestUid = logger.requestUid + "/" + requestUid
	}

	return &gelfLogger{
		writer:     logger.writer,
		facility:   logger.facility,
		hostname:   logger.hostname,
		stderr: logger.stderr,
		requestUid: requestUid,
	}
}

func (logger *gelfLogger) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {

	allExtras := map[string]interface{}{}

	for _, ex := range extras {
		mergo.Merge(&allExtras, ex)
	}

	stdErrMessage := fmt.Sprintf("%s: %s\n", kind, message)

	if logger.requestUid != "" {
		allExtras["request_uid"] = logger.requestUid

		stdErrMessage = fmt.Sprintf("[%s] %s", logger.requestUid, stdErrMessage)
	}

	if logger.stderr {
		os.Stderr.WriteString(stdErrMessage)
	}

	m := &gelf.Message{
		Version:  "1.1",
		Host:     logger.hostname,
		Short:    kind,
		Full:     message,
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
		Level:    level,
		Extra:    allExtras,
		Facility: logger.facility,
	}

	err := logger.writer.WriteMessage(m)
	if err == nil {
		return true
	}

	log.Println("ERROR WriteMessage GELF in GelfWriterLogging.Message:", err.Error())
	if data, err := json.MarshalIndent(extras, "", "    "); err != nil {
		log.Println("WARN log not sent", err)
	} else {
		log.Println("WARN log not sent", string(data))
	}

	return false
}

func (logger *gelfLogger) Write(p []byte) (int, error) {
	if logger.Message(gelf.LOG_INFO, "stdout", strings.Trim(string(p), "\n ")) {
		return len(p), nil
	} else {
		return 0, errors.New("logger.Message() returned false")
	}
}

func (l *gelfLogger) SetAsDefault() Logger {
	defaultLogger = l
	return l
}

func (l *gelfLogger) CopyToStderr() Logger {
	l.stderr = true
	return l
}
