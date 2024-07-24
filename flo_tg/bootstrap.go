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
	LogErrorln("GraylogGELF TCP address:", graylogAddr)

	selfHostname, err := os.Hostname()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Cannot get os.Hostname()"))
	}

	logger := NewGraylogTCPLogger(Graylog_Facility, graylogAddr, selfHostname).SetAsDefault().CopyToStderr()

	mgUri := GetenvStr("MONGO_URI", "mongodb://localhost:27017", true)

	db := NewStorageMongo(mgUri, Mongo_Database, logger)
	if err := db.Ping(); err != nil {
		err = errors.Wrapf(err, "connect to mongodb")
		log.Fatal(err)
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

	logger.Message(gelf.LOG_INFO, "main", fmt.Sprintf("Telegram database is in %s, logs in %s\n", sessionDir, logFilePath))

	return Bootstrap{
		Logger:        logger,
		Storage:       db,
		TgPhone:       phone,
		TgAppId:       appID,
		TgAppHash:     appHash,
		TgWorkFolder:  sessionDir,
		TgLogFileName: logFilePath,
		ServicePort:   servicePort,
		Queue:         NewQueue(0),
	}
}
