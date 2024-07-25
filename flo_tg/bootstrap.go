package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

const (
	Mongo_Database   = "flo_tg"
	Graylog_Facility = "flo_tg"
)

type Bootstrap struct {
	Storage       *Storage
	Logger        Logger
	TgPhone       string
	TgAppId       int
	TgAppHash     string
	TgWorkFolder  string
	TgLogFileName string
	ServicePort   int
	Queue         *Queue
}

func (b *Bootstrap) Close() error {
	return b.Logger.Close()
}

func BootstrapFromEnvironment() Bootstrap {

	servicePort := GetenvInt("FLOTG_PORT", 0, false)

	graylogAddr := GetenvStr("GRAYLOG_ADDRESS", "", false)

	// host name of current container (or system) is used for graylog message "source" field
	selfHostname, err := os.Hostname()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Cannot get os.Hostname()"))
	}

	facility := Graylog_Facility
	if prefix := GetenvStr("LOG_FACILITY_PREFIX", "", true); prefix != "" {
		facility = prefix + "-" + facility
	}

	logger := NewGraylogTCPLogger(facility, graylogAddr, selfHostname).SetAsDefault().CopyToStderr()

	logger.Message(gelf.LOG_DEBUG, "bootstrap", "BootstrapFromEnvironment", GetenvMap(
		"LOG_FACILITY_PREFIX",
		"GRAYLOG_ADDRESS",
		"MONGO_URI",
		"FLOTG_PORT",
		"TG_PHONE",
		"TG_APP_ID",
		"TG_SESSION_PATH",
	))

	mgUri := GetenvStr("MONGO_URI", "mongodb://localhost:27017", true)

	db := NewStorageMongo(mgUri, Mongo_Database, logger)
	if err := db.Ping(); err != nil {
		err = errors.Wrapf(err, "ping mongodb failed")
		logger.Message(gelf.LOG_CRIT, "bootstrap", "Storage failed", map[string]any{
			"err": err,
		})
		os.Exit(1)
	}

	phone := GetenvStr("TG_PHONE", "", false)

	appID := GetenvInt("TG_APP_ID", 0, false)

	appHash := GetenvStr("TG_APP_HASH", "", false)

	sessionsPath := GetenvStr("TG_SESSION_PATH", "", false)

	sessionDir := filepath.Join(sessionsPath, sessionFolder(phone))
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		err = errors.Wrap(err, "Error mkdir (0700) for path "+sessionDir)
		log.Fatal(err)
	}

	logFilePath := filepath.Join(sessionDir, "log.jsonl")

	logger.Message(gelf.LOG_INFO, "bootstrap", fmt.Sprintf("Telegram database is in %s, logs in %s\n", sessionDir, logFilePath))

	return Bootstrap{
		Logger:        logger,
		Storage:       db,
		TgPhone:       phone,
		TgAppId:       appID,
		TgAppHash:     appHash,
		TgWorkFolder:  sessionDir,
		TgLogFileName: logFilePath,
		ServicePort:   servicePort,
		Queue:         NewQueue(200),
	}
}
