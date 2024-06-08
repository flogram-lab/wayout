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

func (handling *telegramHandling) AddHandlers(dispatcher tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(handling.handlerMessage())
	dispatcher.OnNewChannelMessage(handling.handlerChannelMessage())
}

func (handling *telegramHandling) handlerMessage() tg.NewMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		handler := "handlerMessage"

		logInfo := map[string]interface{}{
			"handler":  handler,
			"entities": e,
		}
		logger := handling.bootstrap.Logger

		logInfo["debug_td_msg_type"] = reflect.TypeOf(u.Message).String()
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Message received", logInfo)

		switch msg := u.Message.(type) {

		case *tg.Message:
			return handling.genericHandleMessage(handler, ctx, e, msg)

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

		logInfo := map[string]interface{}{
			"handler":  handler,
			"entities": e,
		}
		logger := handling.bootstrap.Logger

		logInfo["debug_td_msg_type"] = reflect.TypeOf(u.Message).String()
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Channel message received", logInfo)

		switch msg := u.Message.(type) {

		case *tg.Message:
			return handling.genericHandleMessage(handler, ctx, e, msg)

		case *tg.MessageService: // TODO
			// TODO Action : tg.MessageActionGroupCall
			return nil

		default:
			logger.Message(gelf.LOG_WARNING, "telegram_handling", "Message lost! Cast type failed (this should not happen, really)", logInfo)
			return errors.New("message class type not implemented")
		}
	}
}

func (handling *telegramHandling) genericHandleMessage(handler string, ctx context.Context, e tg.Entities, msg *tg.Message) error {

	logInfo := map[string]interface{}{
		"handler":  handler,
		"entities": e,
	}

	logInfo = map[string]interface{}{
		"handler":     handler,
		"entities":    e,
		"from_id":     msg.FromID,
		"message":     msg.Message,
		"peer_id":     msg.PeerID,
		"post_author": msg.PostAuthor,
		"message_uid": msg.ID,
		"is_post":     msg.Post,
	}

	logger := handling.bootstrap.Logger.AddRequestID(fmt.Sprintf("td-client-%d-%s", msg.ID, RandStringBytesMaskImprSrcSB(8)))

	peer, err := storage.FindPeer(ctx, handling.peerDB, msg.GetPeerID())
	if err != nil {

		logger.Message(gelf.LOG_CRIT, "telegram_handling", "Message lost! Peer not found in database", logInfo, map[string]interface{}{
			"err": err.Error(),
		})

		LogErrorln("Chat peer not found in database", msg.GetPeerID())
		return err
	}

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Message received", logInfo)

	source, deepFromId := handling.converter.makeProtoSource(msg, peer, e, handling.selfUser)

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "After makeProtoSource", logInfo, map[string]interface{}{
		"debug_rpc": handling.converter.encodeToJson(source, false),
	})

	message := handling.converter.makeProtoMessage(msg, source, deepFromId)

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "After makeProtoMessage", logInfo, map[string]interface{}{
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
		logger.Message(gelf.LOG_CRIT, "telegram_handling", "Source storage failed", logInfo, map[string]interface{}{
			"err": err.Error(),
		})
		return err
	} else {
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Source saved", logInfo)
	}

	messageRefId, err := save.Message(ctx, handling.converter, source, message)
	logInfo["message_ref_id"] = messageRefId

	if err != nil {
		logger.Message(gelf.LOG_CRIT, "telegram_handling", "Message storage failed", logInfo, map[string]interface{}{
			"err": err.Error(),
		})
		return err
	} else {
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Message saved", logInfo)
	}

	return err
}
