/***************************************************************************************
**** The code in this file was mostly copy-pasted from official tdlib-go demo.		****
**** It was updated to only call some functions from other code files to extend it.	****
***************************************************************************************/

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	pebbledb "github.com/cockroachdb/pebble"
	"github.com/go-faster/errors"
	boltstor "github.com/gotd/contrib/bbolt"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/contrib/pebble"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/examples"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
	lj "gopkg.in/natefinch/lumberjack.v2"
)

func CreateAndRunTelegramClient(ctx context.Context, bootstrap Bootstrap) error {

	// Setting up logging to file with rotation.
	//
	// TODO: WTF? remove double logging in this
	//
	// Log to file, so we don't interfere with prompts and messages to user.
	logWriter := zapcore.AddSync(&lj.Logger{
		Filename:   bootstrap.TgLogFileName,
		MaxBackups: 3,
		MaxSize:    1, // megabytes
		MaxAge:     7, // days
	})
	logCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		logWriter,
		zap.DebugLevel,
	)
	lg := zap.New(logCore)
	defer func() { _ = lg.Sync() }()

	var err error
	var db *pebbledb.DB

	// So, we are storing session information in current directory, under subdirectory "session/phone_hash"
	sessionStorage := &telegram.FileSessionStorage{
		Path: filepath.Join(bootstrap.TgWorkFolder, "session.json"),
	}
	// Peer storage, for resolve caching and short updates handling.
	db, err = pebbledb.Open(filepath.Join(bootstrap.TgWorkFolder, "peers.pebble.db"), &pebbledb.Options{})
	if err != nil {
		return errors.Wrap(err, "create pebble storage")
	}
	peerDB := pebble.NewPeerStorage(db)
	lg.Info("Storage", zap.String("path", bootstrap.TgWorkFolder))

	// Setting up client.
	//
	// Dispatcher is used to register handlers for events.
	dispatcher := tg.NewUpdateDispatcher()
	// Setting up update handler that will fill peer storage before
	// calling dispatcher handlers.
	updateHandler := storage.UpdateHook(dispatcher, peerDB)

	// Setting up persistent storage for qts/pts to be able to
	// recover after restart.
	boltdb, err := bbolt.Open(filepath.Join(bootstrap.TgWorkFolder, "updates.bolt.db"), 0666, nil)
	if err != nil {
		return errors.Wrap(err, "create bolt storage")
	}
	updatesRecovery := updates.New(updates.Config{
		Handler: updateHandler, // using previous handler with peerDB
		Logger:  lg.Named("updates.recovery"),
		Storage: boltstor.NewStateStorage(boltdb),
	})

	// Handler of FLOOD_WAIT that will automatically retry request.
	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		// Notifying about flood wait.
		lg.Warn("Flood wait", zap.Duration("wait", wait.Duration))
		bootstrap.Logger.Message(gelf.LOG_WARNING, "telegram", fmt.Sprintf("Got FLOOD_WAIT. Will retry after %s", wait.Duration))
	})

	// Filling client options.
	options := telegram.Options{
		Logger:         lg,              // Passing logger for observability.
		SessionStorage: sessionStorage,  // Setting up session sessionStorage to store auth data.
		UpdateHandler:  updatesRecovery, // Setting up handler for updates from server.
		Middlewares: []telegram.Middleware{
			// Setting up FLOOD_WAIT handler to automatically wait and retry request.
			waiter,
			// Setting up general rate limits to less likely get flood wait errors.
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
		},
	}
	client := telegram.NewClient(bootstrap.TgAppId, bootstrap.TgAppHash, options)
	api := client.API()

	// Authentication flow handles authentication process, like prompting for code and 2FA password.
	flow := auth.NewFlow(examples.Terminal{PhoneNumber: bootstrap.TgPhone}, auth.SendCodeOptions{})

	return waiter.Run(ctx, func(ctx context.Context) error {

		// Install panic handler with logging on this thread/goroutine
		defer LogPanicErr(&err, bootstrap.Logger, "telegram", "waiter.Run")

		// Spawning main goroutine.
		if err = client.Run(ctx, func(ctx context.Context) error {

			// Install panic handler with logging on this thread/goroutine
			defer LogPanicErr(&err, bootstrap.Logger, "telegram", "client.Run")

			// Perform auth if no session is available.
			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				return errors.Wrap(err, "auth")
			}

			// Getting info about current user.
			self, err := client.Self(ctx)
			if err != nil {
				return errors.Wrap(err, "call self")
			}

			name := self.FirstName
			if self.LastName != "" {
				name = fmt.Sprintf("%s, %s", name, self.LastName)
			}
			if self.Username != "" {
				name = fmt.Sprintf("%s, @%s", name, self.Username)
			}
			bootstrap.Logger.Message(gelf.LOG_INFO, "telegram", fmt.Sprintf("Current user: %s, %d\n", name, self.ID))

			lg.Info("Login",
				zap.String("first_name", self.FirstName),
				zap.String("last_name", self.LastName),
				zap.String("username", self.Username),
				zap.Int64("id", self.ID),
			)

			handling := newTelegramHandling(bootstrap, peerDB, self)
			handling.AddEventHandlers(dispatcher)

			// Waiting until context is done.
			bootstrap.Logger.Message(gelf.LOG_DEBUG, "telegram", "Listening for updates. Interrupt (Ctrl+C) to stop.")

			return updatesRecovery.Run(ctx, api, self.ID, updates.AuthOptions{
				IsBot: self.Bot,
				OnStart: func(ctx context.Context) {
					handling.bootstrap.Logger.Message(gelf.LOG_INFO, "telegram", "Update recovery initialized and started, listening for events")
				},
			})
		}); err != nil {
			return errors.Wrap(err, "run")
		}

		return err
	})
}
