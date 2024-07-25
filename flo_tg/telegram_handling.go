package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-faster/errors"
	peeble "github.com/gotd/contrib/pebble"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/tg"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type telegramHandling struct {
	bootstrap Bootstrap
	peerDB    *peeble.PeerStorage
	converter *converter
	selfUser  *tg.User
}

func newTelegramHandling(bootstrap Bootstrap, peerDB *peeble.PeerStorage, selfUser *tg.User) *telegramHandling {
	return &telegramHandling{
		bootstrap: bootstrap,
		peerDB:    peerDB,
		converter: newConverter(bootstrap),
		selfUser:  selfUser,
	}
}

func (handling *telegramHandling) Attach(dispatcher tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(handling.handlerMessage())
	dispatcher.OnNewChannelMessage(handling.handlerChannelMessage())
}

func (handling *telegramHandling) requestFromMessage(logInfo map[string]any, msg tg.MessageClass) (Logger, error) {
	logger := handling.bootstrap.Logger

	if msg == nil {
		handling.bootstrap.Logger.Message(gelf.LOG_ERR, "telegram_handling", "Message is nil", logInfo)
		return logger, errors.New("Update.Message is nil")
	}

	logger = logger.AddRequestID(fmt.Sprintf("td-message-%d-%s", msg.GetID(), RandStringBytesMaskImprSrcSB(8)))

	logInfo["debug_td_msg_type"] = reflect.TypeOf(msg).String()
	logInfo["debug_td_msg_type_tlid"] = msg.TypeID()

	return logger, nil
}

func (handling *telegramHandling) handlerMessage() tg.NewMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		handler := "handlerMessage"
		logInfo := map[string]any{
			"handler":              handler,
			"entities":             e,
			"debug_td_update_type": reflect.TypeOf(u).String(),
		}

		var (
			err    error
			logger Logger = handling.bootstrap.Logger
		)

		defer LogPanicErr(&err, logger, "telegram_handling", handler)

		logger, err = handling.requestFromMessage(logInfo, u.Message)
		if err != nil {
			return err
		}

		switch msg := u.Message.(type) {

		case *tg.Message:
			logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Handling (message) as genericHandleMessage", logInfo)

			// if msg.Message == "crash!" {
			// 	panic("testing panic 2")
			// }

			handling.bootstrap.Queue.Enqueue(func(ctx context.Context) {
				handling.genericHandleMessage(handler, ctx, e, msg, logger)
			})
			return nil

		case *tg.MessageService: // TODO
			// TODO Action : tg.MessageActionGroupCall
			return nil

		default:
			logger.Message(gelf.LOG_WARNING, "telegram_handling", "Message lost! Cast type failed (this should not happen, really)", logInfo)
			return errors.New("message class type not implemented")
		}
	}
}

func (handling *telegramHandling) handlerChannelMessage() tg.NewChannelMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {

		handler := "handlerChannelMessage"
		logInfo := map[string]any{
			"handler":              handler,
			"entities":             e,
			"debug_td_update_type": reflect.TypeOf(u).String(),
		}

		var (
			err    error
			logger Logger = handling.bootstrap.Logger
		)

		defer LogPanicErr(&err, logger, "telegram_handling", handler)

		logger, err = handling.requestFromMessage(logInfo, u.Message)
		if err != nil {
			return err
		}

		switch msg := u.Message.(type) {

		case *tg.Message:
			logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Handling (channel message) as genericHandleMessage", logInfo)
			var op Op = func(ctx context.Context) {
				handling.genericHandleMessage(handler, ctx, e, msg, logger)
			}
			handling.bootstrap.Queue.Enqueue(op)
			return nil

		case *tg.MessageService: // TODO
			// TODO Action : tg.MessageActionGroupCall
			return nil

		default:
			logger.Message(gelf.LOG_WARNING, "telegram_handling", "Message lost! Cast type failed (this should not happen, really)", logInfo)
			return errors.New("message class type not implemented")
		}
	}
}

func (handling *telegramHandling) genericHandleMessage(handler string, ctx context.Context, e tg.Entities, msg *tg.Message, logger Logger) error {

	logInfo := map[string]any{
		"handler":     handler,
		"entities":    e,
		"from_id":     msg.FromID,
		"message":     msg.Message,
		"peer_id":     msg.PeerID,
		"post_author": msg.PostAuthor,
		"message_id":  msg.ID,
		"is_post":     msg.Post,
	}

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "genericHandleMessage", logInfo)

	peer, err := storage.FindPeer(ctx, handling.peerDB, msg.GetPeerID())
	if err != nil {

		logger.Message(gelf.LOG_CRIT, "telegram_handling", "Message lost! Peer not found in database", logInfo, map[string]any{
			"err": err.Error(),
		})

		return err
	}

	source, deepFromId := handling.converter.makeProtoSource(msg, peer, e, handling.selfUser)

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "After makeProtoSource", logInfo, map[string]any{
		"debug_rpc": handling.converter.encodeToJson(source, false),
	})

	message := handling.converter.makeProtoMessage(msg, source, deepFromId)

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "After makeProtoMessage", logInfo, map[string]any{
		"debug_rpc": handling.converter.encodeToJson(message, false),
	})

	logInfo["source_uid"] = source.SourceUid
	logInfo["message_uid"] = message.MessageUid
	logInfo["deepFromId"] = deepFromId

	save := storageSave{
		storage: handling.bootstrap.Storage,
		logger:  logger,
	}

	sourceRefId, err := save.Source(ctx, handling.converter, source)
	logInfo["sourceRefId"] = sourceRefId

	if err != nil {
		logger.Message(gelf.LOG_CRIT, "telegram_handling", "Source storage failed", logInfo, map[string]any{
			"err": err.Error(),
		})
		return err
	} else {
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Source saved", logInfo)
	}

	messageRefId, err := save.Message(ctx, handling.converter, source, message)
	logInfo["message_ref_id"] = messageRefId

	if err != nil {
		logger.Message(gelf.LOG_CRIT, "telegram_handling", "Message storage failed", logInfo, map[string]any{
			"err": err.Error(),
		})
	} else {
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Message saved", logInfo)
	}

	return err
}
