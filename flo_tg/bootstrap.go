package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/go-faster/errors"
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
}

func (b *Bootstrap) Close() error {
	return b.Logger.Close()
}

func BootstrapFromEnvironment() Bootstrap {
	servicePort := GetenvInt("FLOTG_PORT", 0, false)

	var logging Logger = &dummyLogging{}

	graylogAddr := GetenvStr("GRAYLOG_ADDRESS", "", false)
	LogErrorln("GraylogGELF TCP address:", graylogAddr)

	selfHostname, err := os.Hostname()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Cannot get os.Hostname()"))
	}

	logging = NewGraylogTCPLogger(Graylog_Facility, graylogAddr, selfHostname).SetAsDefault()

	mgUri := GetenvStr("MONGO_URI", "mongodb://localhost:27017", true)

	db := NewStorageMongo(mgUri, Mongo_Database)
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

	LogErrorf("Telegram database is in %s, logs in %s\n", sessionDir, logFilePath)

	return Bootstrap{
		Logger:        logging,
		Storage:       db,
		TgPhone:       phone,
		TgAppId:       appID,
		TgAppHash:     appHash,
		TgWorkFolder:  sessionDir,
		TgLogFileName: logFilePath,
		ServicePort:   servicePort,
	}
}
