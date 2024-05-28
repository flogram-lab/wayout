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
}

func newTelegramHandling(bootstrap Bootstrap, peerDB *peeble.PeerStorage) *telegramHandling {
	return &telegramHandling{
		bootstrap: bootstrap,
		peerDB:    peerDB,
	}
}

func (handling *telegramHandling) AddHandlers(dispatcher tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(handling.handlerMessage())
	dispatcher.OnNewChannelMessage(handling.handlerChannelMessage())
}

func (handling *telegramHandling) handlerMessage() tg.NewMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		log.Println("User message received!")
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}

		hasPeer := true
		_, err := storage.FindPeer(ctx, handling.peerDB, msg.GetPeerID())
		if err != nil {
			hasPeer = false
			log.Println("User Peer not found in database", msg.GetPeerID())
		}

		log.Println("Message FromID", msg.FromID, msg.Message)

		handling.bootstrap.Logging.Message(gelf.LOG_WARNING, "Telegram", "User message", map[string]interface{}{
			"msg_from_id": msg.FromID,
			"has_peer":    hasPeer,
		})

		return nil
	}
}

func (handling *telegramHandling) handlerChannelMessage() tg.NewChannelMessageHandler {
	return func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		log.Println("Channel message received!")
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}

		hasPeer := true
		_, err := storage.FindPeer(ctx, handling.peerDB, msg.GetPeerID())
		if err != nil {
			hasPeer = false
			log.Println("Channel Peer not found in database", msg.GetPeerID())
		}

		log.Println("Message FromID", msg.FromID, msg.Message)

		handling.bootstrap.Logging.Message(gelf.LOG_WARNING, "Telegram", "Channel message", map[string]interface{}{
			"msg_from_id": msg.FromID,
			"has_peer":    hasPeer,
		})

		return nil
	}
}
