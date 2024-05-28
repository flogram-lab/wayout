package main

import (
	"io"
	"log"
	"os"
	"time"

	"dario.cat/mergo"
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
	writer gelf.Writer
}

func NewLoggingGraylogTCP(facility string) Logging {
	graylogAddr := os.Getenv("GRAYLOG_ADDRESS")
	if graylogAddr == "" {
		log.Println("WARN not using Graylog, empty", "GRAYLOG_ADDRESS")
		return dummyLogging{}
	}

	log.Println("GraylogGELF TCP address:", graylogAddr)

	gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
	if err != nil {
		log.Fatalf("gelf.NewTCPWriter: %s", err)
	}

	gelfWriter.Facility = facility

	// log to both stderr and graylog2
	log.SetOutput(io.MultiWriter(os.Stderr, gelfWriter))
	log.Printf("logging to stderr & graylog2@'%s'", graylogAddr)

	return &GelfWriterLogging{writer: gelfWriter}
}

func (_ dummyLogging) Close() error {
	return nil
}

func (_ dummyLogging) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {
	return false
}

func (self *GelfWriterLogging) Close() error {
	return self.writer.Close()
}

func (self *GelfWriterLogging) Message(level int32, kind string, message string, extras ...map[string]interface{}) bool {

	allExtras := map[string]interface{}{}
	for _, ex := range extras {
		mergo.Merge(&allExtras, ex)
	}

	m := &gelf.Message{
		Version:  "1.0",
		Short:    kind,
		Full:     message,
		TimeUnix: float64(time.Now().Unix()),
		Level:    level,
		Extra:    allExtras,
	}

	err := self.writer.WriteMessage(m)
	if err == nil {
		return true
	}

	log.Println("Error writing GELF message in GelfWriterLogging.Message:", err.Error())

	return false
}
