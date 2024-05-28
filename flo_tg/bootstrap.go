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
	Logging       Logging
	TgPhone       string
	TgAppId       int
	TgAppHash     string
	TgWorkFolder  string
	TgLogFileName string
	ServicePort   int
}

func BootstrapFromEnvironment() Bootstrap {
	servicePort := GetenvInt("FLOTG_PORT", 0, false)

	logging := NewLoggingGraylogTCP(Graylog_Facility)
	defer logging.Close()

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

	log.Printf("Telegram database is in %s, logs in %s\n", sessionDir, logFilePath)

	return Bootstrap{
		Logging:       logging,
		Storage:       db,
		TgPhone:       phone,
		TgAppId:       appID,
		TgAppHash:     appHash,
		TgWorkFolder:  sessionDir,
		TgLogFileName: logFilePath,
		ServicePort:   servicePort,
	}
}
