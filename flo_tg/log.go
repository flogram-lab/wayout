package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type Logging interface {
	Close() error
	Message(level int32, kind string, message string, extras ...map[string]interface{}) bool
}

type dummyLogging struct {
	Logging
}

type GelfWriterLogging struct {
	Logging
	writer             gelf.Writer
	facility, hostname string
}

func NewLoggingGraylogTCP(facility string) Logging {
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

	// log to both stderr and graylog2
	log.SetOutput(io.MultiWriter(os.Stderr, gelfWriter))
	log.Printf("logging to stderr & graylog2@'%s'", graylogAddr)

	return &GelfWriterLogging{
		writer:   gelfWriter,
		facility: facility,
		hostname: hostname,
	}
}

func (dummyLogging) Close() error {
	return nil
}

func (dummyLogging) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {
	if data, err := json.MarshalIndent(extras, "", "    "); err != nil {
		log.Println("WARN log not sent", err)
	} else {
		log.Println("WARN log not sent", string(data))
	}

	return true
}

func (logging *GelfWriterLogging) Close() error {
	return logging.writer.Close()
}

func (self *GelfWriterLogging) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {

	allExtras := map[string]interface{}{}
	for _, ex := range extras {
		mergo.Merge(&allExtras, ex)
	}

	m := &gelf.Message{
		Version:  "1.1",
		Host:     self.hostname,
		Short:    kind,
		Full:     message,
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
		Level:    level,
		Extra:    allExtras,
		Facility: self.facility,
	}

	err := self.writer.WriteMessage(m)
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
