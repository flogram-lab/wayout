package main

import (
	"context"
	"log"

	peeble "github.com/gotd/contrib/pebble"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/tg"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type telegramHandling struct {
	bootstrap Bootstrap
	peerDB    *peeble.PeerStorage
	converter *converter
}

func newTelegramHandling(bootstrap Bootstrap, peerDB *peeble.PeerStorage) *telegramHandling {
	return &telegramHandling{
		bootstrap: bootstrap,
		peerDB:    peerDB,
		converter: newConverter(bootstrap),
	}
}

func (handling *telegramHandling) AddHandlers(dispatcher tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(handling.handlerMessage())
	dispatcher.OnNewChannelMessage(handling.handlerChannelMessage())
}

func (handling *telegramHandling) handlerMessage() tg.NewMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		return handling.genericHandleMessage("new message handler", ctx, e, u)
	}
}

func (handling *telegramHandling) handlerChannelMessage() tg.NewChannelMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		return handling.genericHandleMessage("channel message handler", ctx, e, u)
	}
}


func (handling *telegramHandling) genericHandleMessage(handler string, ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {

	logInfo := map[string]interface{}{
		"handler":  handler,
		"entities": e,
	}

	handling.bootstrap.Logging.Message(gelf.LOG_DEBUG, "Telegram", "Message received", logInfo)

	msg, ok := u.Message.(*tg.Message)
	if !ok {

		handling.bootstrap.Logging.Message(gelf.LOG_ERR, "Telegram", "Message lost! Cast type failed (this should not happen, really)", logInfo)

		return nil
	}

	logInfo = map[string]interface{}{
		"handler":    handler,
		"entities": e,
		"from_id":     msg.FromID,
		"message":     msg.Message,
		"peer_id":     msg.PeerID,
		"post_author": msg.PostAuthor,
		"message_uid": msg.ID,
		"is_post":     msg.Post,
	}

	peer, err := storage.FindPeer(ctx, handling.peerDB, msg.GetPeerID())
	if err != nil {

		handling.bootstrap.Logging.Message(gelf.LOG_CRIT, "Telegram", "Message lost! Peer not found in database", logInfo, map[string]interface{}{
			"err": err.Error(),
		})

		log.Println("Chat peer not found in database", msg.GetPeerID())
		return err
	}

	handling.bootstrap.Logging.Message(gelf.LOG_DEBUG, "Telegram", "Message received", logInfo)

	source := handling.converter.makeSource(msg, peer, e)
	message := handling.converter.makeMessage(msg, source)

	logInfo["source_uid"] = source.SourceUID;
	logInfo["message_uid"] = message.MessageUID;

	err = handling.bootstrap.Storage.saveMessage(source, message)

	if err != nil {
		handling.bootstrap.Logging.Message(gelf.LOG_ERROR, "Telegram", "Message storage failed", logInfo, map[string]interface{}{
			"err": err.Error(),
		})
	}

	handling.bootstrap.Logging.Message(gelf.LOG_DEBUG, "Telegram", "Message saved", logInfo)

	return err
}