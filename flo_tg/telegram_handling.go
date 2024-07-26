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

func (handling *telegramHandling) AddEventHandlers(dispatcher tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(
		telegramHandlerWrapper[*tg.UpdateNewMessage]{handling}.
			wrappedRequestOnQueue("newMessage",
				func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage, event string, logger Logger) error {
					msg := u.Message

					logger.SetFields(map[string]any{
						"debug_td_msg_type":      reflect.TypeOf(msg).String(),
						"debug_td_msg_type_tlid": msg.TypeID(),
					})
					logger.Message(gelf.LOG_DEBUG, "telegram_handling", fmt.Sprintf("Message ID: %d", msg.GetID()))

					switch msg := u.Message.(type) {

					case *tg.Message:
						handling.saveMessageGeneric(ctx, event, e, msg, logger)
						return nil

					case *tg.MessageService: // TODO
						// TODO Action : tg.MessageActionGroupCall
						return nil

					default:
						logger.Message(gelf.LOG_ALERT, "telegram_handling", "Message lost! Cast type failed (this should not happen, really)")
						return errors.New("message class type not implemented")
					}
				}))

	dispatcher.OnNewChannelMessage(
		telegramHandlerWrapper[*tg.UpdateNewChannelMessage]{handling}.
			wrappedRequestOnQueue("newChannelMessage",
				func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage, event string, logger Logger) error {
					msg := u.Message

					logger.SetFields(map[string]any{
						"debug_td_msg_type":      reflect.TypeOf(msg).String(),
						"debug_td_msg_type_tlid": msg.TypeID(),
					})
					logger.Message(gelf.LOG_DEBUG, "telegram_handling", fmt.Sprintf("Message ID: %d", msg.GetID()))

					switch msg := u.Message.(type) {

					case *tg.Message:
						handling.saveMessageGeneric(ctx, event, e, msg, logger)
						return nil

					case *tg.MessageService: // TODO
						// TODO Action : tg.MessageActionGroupCall
						return nil

					default:
						logger.Message(gelf.LOG_ALERT, "telegram_handling", "Message lost! Cast type failed (this should not happen, really)")
						return errors.New("message class type not implemented")
					}
				}))
}

func (handling *telegramHandling) saveMessageGeneric(ctx context.Context, event string, e tg.Entities, msg *tg.Message, logger Logger) {
	logger.SetFields(map[string]any{
		"event":       event,
		"entities":    e,
		"from_id":     msg.FromID,
		"message":     msg.Message,
		"peer_id":     msg.PeerID,
		"post_author": msg.PostAuthor,
		"message_id":  msg.ID,
		"is_post":     msg.Post,
	})

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Handler ("+event+") entered saveMessageGeneric")

	peer, err := storage.FindPeer(ctx, handling.peerDB, msg.GetPeerID())
	if err != nil {

		logger.Message(gelf.LOG_ALERT, "telegram_handling", "Message lost! Peer not found in database", map[string]any{
			"err": err.Error(),
		})

		return
	}

	if msg.Message == "crash!" {
		panic("testing crash from message")
	}

	source, deepFromId := handling.converter.makeProtoSource(msg, peer, e, handling.selfUser)

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "After makeProtoSource", map[string]any{
		"debug_rpc": handling.converter.encodeToJson(source, false),
	})

	message := handling.converter.makeProtoMessage(msg, source, deepFromId)

	logger.Message(gelf.LOG_DEBUG, "telegram_handling", "After makeProtoMessage", map[string]any{
		"debug_rpc": handling.converter.encodeToJson(message, false),
	})

	logger.SetField("source_uid", source.SourceUid)
	logger.SetField("message_uid", message.MessageUid)
	logger.SetField("deepFromId", deepFromId)

	save := storageSave{ // TODO: move in ctx here
		storage: handling.bootstrap.Storage,
		logger:  logger,
	}

	sourceRefId, err := save.Source(ctx, handling.converter, source)
	logger.SetField("sourceRefId", sourceRefId)

	if err != nil {
		logger.Message(gelf.LOG_ALERT, "telegram_handling", "Source storage failed", map[string]any{
			"err": err.Error(),
		})
		return
	} else {
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Source saved")
	}

	messageRefId, err := save.Message(ctx, handling.converter, source, message)
	logger.SetField("message_ref_id", messageRefId)

	if err != nil {
		logger.Message(gelf.LOG_ALERT, "telegram_handling", "Message storage failed", map[string]any{
			"err": err.Error(),
		})
	} else {
		logger.Message(gelf.LOG_DEBUG, "telegram_handling", "Message saved")
	}
}
