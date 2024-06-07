package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type Logger interface {
	io.Writer
	Close() error
	Message(level int32, kind string, message string, extras ...map[string]interface{}) bool
	AddRequestID(requestUid string) Logger
}

type dummyLogging struct {
	Logger
}

type gelfLogger struct {
	Logger
	writer                         gelf.Writer
	facility, hostname, requestUid string
}

func newLoggerGraylogTCP(facility string) Logger {
	graylogAddr := os.Getenv("GRAYLOG_ADDRESS")
	if graylogAddr == "" {
		log.Println("WARN not using Graylog, empty", "GRAYLOG_ADDRESS")
		return dummyLogging{}
	}

	log.Println("GraylogGELF TCP address:", graylogAddr)

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Cannot get os.Hostname()"))
	}

	gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
	if err != nil {
		log.Fatalf("gelf.NewTCPWriter: %s", err)
	}

	gelfWriter.Facility = facility

	logger := &gelfLogger{
		writer:     gelfWriter,
		facility:   facility,
		hostname:   hostname,
		requestUid: "",
	}

	// log to both stderr and graylog2
	log.SetOutput(io.MultiWriter(os.Stdout, logger))
	log.Printf("logging to stdout & graylog @%s", graylogAddr)

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

func (logger *gelfLogger) Write(p []byte) (int, error) {
	if logger.Message(gelf.LOG_INFO, "stdout", strings.Trim(string(p), "\n ")) {
		return len(p), nil
	} else {
		return 0, errors.New("logger.Message() returned false")
	}
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
		requestUid: requestUid,
	}
}

func (logger *gelfLogger) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {

	allExtras := map[string]interface{}{}

	for _, ex := range extras {
		mergo.Merge(&allExtras, ex)
	}

	if logger.requestUid != "" {
		allExtras["request_uid"] = logger.requestUid
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
