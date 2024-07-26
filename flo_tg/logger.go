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
	Message(level int32, kind string, message string, extras ...map[string]any) bool
	AddRequestID(requestUid string, fields ...map[string]any) Logger
	SetField(key string, value any)
	SetFields(map[string]any)
	SetAsDefault() Logger
}

type dummyLogging struct {
	Logger
}

type gelfLogger struct {
	Logger
	writer             gelf.Writer
	facility, hostname string
	fields             map[string]any
	stderr             bool
}

func NewGraylogTCPLogger(facility, graylogAddr, selfHostname string) Logger {

	gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
	if err != nil {
		log.Fatalf("gelf.NewTCPWriter: %s", err)
	}

	gelfWriter.Facility = facility

	logger := &gelfLogger{
		writer:   gelfWriter,
		facility: facility,
		hostname: selfHostname,
		stderr:   true,
		fields:   map[string]any{},
	}

	log.Printf("Logging errors to stderr, full logging to  graylog @%s", graylogAddr)

	return logger
}

func (dummyLogging) Close() error {
	return nil
}

func (dummyLogging) Message(level int32, kind string, message string, extras ...map[string]any) bool {
	if data, err := json.MarshalIndent(extras, "", "    "); err != nil {
		log.Println("WARN log not sent", level, kind, message)
	} else {
		log.Println("WARN log not sent", level, kind, message, string(data))
	}

	return true
}

func (dummy dummyLogging) AddRequestID(string, ...map[string]any) Logger {
	return dummy
}

func (dummyLogging) SetField(string, any) {
}

func (dummyLogging) SetFields(map[string]any) {
}

func (dummy dummyLogging) Write(p []byte) (int, error) {
	return 0, nil
}

func (dummy dummyLogging) SetAsDefault() Logger {
	return dummy
}

func (logger *gelfLogger) Close() error {
	return logger.writer.Close()
}

func (logger *gelfLogger) AddRequestID(requestUid string, fields ...map[string]any) Logger {
	if oldId, ok := logger.fields["request_uid"]; ok {
		requestUid = oldId.(string) + "/" + requestUid
	}

	newFields := map[string]any{}
	mergo.Merge(&newFields, logger.fields, mergo.WithOverride)

	for _, v := range fields {
		mergo.Merge(&newFields, v, mergo.WithOverride)
	}

	newFields["request_uid"] = requestUid

	return &gelfLogger{
		writer:   logger.writer,
		facility: logger.facility,
		hostname: logger.hostname,
		stderr:   logger.stderr,
		fields:   newFields,
	}
}

func (logger *gelfLogger) SetField(key string, value any) {
	logger.fields[key] = value
}

func (logger *gelfLogger) SetFields(newFields map[string]any) {
	mergo.Merge(&logger.fields, newFields, mergo.WithOverride)
}

func (logger *gelfLogger) Message(level int32, kind string, message string, fields ...map[string]any) bool {

	messageFields := logger.fields

	if len(fields) > 0 {
		messageFields = make(map[string]any)

		mergo.Merge(&messageFields, logger.fields, mergo.WithOverride)

		for _, callExtraFields := range fields {
			mergo.Merge(&messageFields, callExtraFields, mergo.WithOverride)
		}
	}

	if level <= gelf.LOG_ERR {
		stdErrMessage := fmt.Sprintf("%s: %s\n", kind, message)

		if ruid, ok := logger.fields["request_uid"].(string); ok && ruid != "" {
			stdErrMessage = fmt.Sprintf("[%s] %s", ruid, stdErrMessage)
		}

		os.Stderr.WriteString(stdErrMessage)
	}

	m := &gelf.Message{
		Version:  "1.1",
		Host:     logger.hostname,
		Short:    kind,
		Full:     message,
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
		Level:    level,
		Extra:    messageFields,
		Facility: logger.facility,
	}

	err := logger.writer.WriteMessage(m)
	if err == nil {
		return true
	}

	log.Println("ERROR WriteMessage GELF in GelfWriterLogging.Message:", err.Error())
	if data, err := json.MarshalIndent(fields, "", "    "); err != nil {
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
