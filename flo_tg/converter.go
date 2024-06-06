package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/tg"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type converter struct {
	bootstrap Bootstrap
}

func newConverter(bootstrap Bootstrap) *converter {
	return &converter{
		bootstrap: bootstrap,
	}
}

func (c *converter) makeProtoSource(msg *tg.Message, peer storage.Peer, e tg.Entities, toUser *tg.User) *proto.FLO_SOURCE {

	// TODO: proto add detection for username

	// TODO: Add proto flags: scam, bot, fake, premium
	if peer.Channel != nil {
		v := &proto.FLO_SOURCE{
			Flags:      int32(proto.FLAGS_V1) | int32(proto.FLAGS_Tg) | int32(proto.FLAGS_Channel),
			DeepFromId: peer.Channel.ID,
			SourceUid:  fmt.Sprintf("tgv1-fromid-%d", peer.Channel.ID),
			Title:      peer.Channel.Title,
		}

		if peer.Channel.Username != "" {
			// TODO: v.Flags |= int32(proto.FLAGS_SourceUsername)
			// TODO: v.Username = fmt.Sprintf("https://t.me/@%s", peer.Channel.Username)
		}

		return v

	} else if peer.User != nil {

		v := &proto.FLO_SOURCE{
			Flags:      int32(proto.FLAGS_V1) | int32(proto.FLAGS_Tg) | int32(proto.FLAGS_User),
			DeepFromId: peer.User.ID,
			SourceUid:  fmt.Sprintf("tgv1-fromid-%d", peer.User.ID),
			Title:      strings.Trim(fmt.Sprintf("%s %s", peer.User.FirstName, peer.User.LastName), " "),
		}

		if peer.User.Username != "" {
			// TODO: v.Flags |= int32(proto.FLAGS_SourceUsername)
			// TODO: v.Username = fmt.Sprintf("https://t.me/@%s", peer.User.Username)
		}

		return v

	} else if peer.Chat != nil {

		v := &proto.FLO_SOURCE{
			Flags:      int32(proto.FLAGS_V1) | int32(proto.FLAGS_Tg) | int32(proto.FLAGS_Group),
			DeepFromId: peer.Chat.ID,
			SourceUid:  fmt.Sprintf("tgv1-fromid-%d", peer.Chat.ID),
			Title:      peer.Channel.Title,
		}

		// TODO: No peer.Chat.Username, how to get group chat username (for a public group)

		return v
	}

	return &proto.FLO_SOURCE{
		Flags: int32(proto.FLAGS_Invalid),
	}
}

func (c *converter) makeProtoMessage(msg *tg.Message, source *proto.FLO_SOURCE) *proto.FLO_MESSAGE {

	messageDeepLinks := []string{
		fmt.Sprintf("https://t.me/c/%d/%d", source.DeepFromId, msg.ID),
	}

	// TODO: proto add detection for username
	// if source.Flags&int32(proto.FLAGS_SourceUsername) != 0 {
	// 	s := fmt.Sprintf("https://t.me/%s/%d", source.Username, msg.ID)
	// 	messageDeepLinks = append(messageDeepLinks, s)
	// }

	return &proto.FLO_MESSAGE{
		Flags:        source.Flags,
		CreatedAt:    timestamppb.New(time.Unix(int64(msg.Date), 0)),
		Title:        source.Title,
		DeepFromId:   source.DeepFromId,
		SourceUid:    source.SourceUid,
		MessageUid:   fmt.Sprintf("%s-%d", source.SourceUid, msg.ID),
		Text:         msg.Message,
		MessageLinks: messageDeepLinks,
	}
}
