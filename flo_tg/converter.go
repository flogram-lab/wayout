package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	protobuf "github.com/gogo/protobuf/proto"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/tg"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type converter struct {
	bootstrap Bootstrap
}

func newConverter(bootstrap Bootstrap) *converter {
	return &converter{
		bootstrap: bootstrap,
	}
}

func (c *converter) makeProtoSource(_ *tg.Message, peer storage.Peer, _ tg.Entities, _ *tg.User) (*proto.FLO_SOURCE, int64) {

	// TODO: proto add detection for username

	// TODO: Add proto flags: scam, bot, fake, premium
	if peer.Channel != nil {
		v := &proto.FLO_SOURCE{
			Flags:     int32(proto.FLAGS_V1) | int32(proto.FLAGS_Tg) | int32(proto.FLAGS_Channel),
			SourceUid: fmt.Sprintf("tgv1-fromid-%d", peer.Channel.ID),
			Title:     peer.Channel.Title,
		}

		if peer.Channel.Username != "" {
			// TODO: v.Flags |= int32(proto.FLAGS_TgUsername)
			// TODO: v.Username = fmt.Sprintf("https://t.me/@%s", peer.Channel.Username)
		}

		return v, peer.Channel.ID

	} else if peer.User != nil {

		v := &proto.FLO_SOURCE{
			Flags:     int32(proto.FLAGS_V1) | int32(proto.FLAGS_Tg) | int32(proto.FLAGS_User),
			SourceUid: fmt.Sprintf("tgv1-fromid-%d", peer.User.ID),
			Title:     strings.Trim(fmt.Sprintf("%s %s", peer.User.FirstName, peer.User.LastName), " "),
		}

		if peer.User.Username != "" {
			// TODO: v.Flags |= int32(proto.FLAGS_TgUsername)
			// TODO: v.Username = fmt.Sprintf("https://t.me/@%s", peer.User.Username)
		}

		return v, peer.User.ID

	} else if peer.Chat != nil {

		v := &proto.FLO_SOURCE{
			Flags:     int32(proto.FLAGS_V1) | int32(proto.FLAGS_Tg) | int32(proto.FLAGS_Group),
			SourceUid: fmt.Sprintf("tgv1-fromid-%d", peer.Chat.ID),
			Title:     peer.Channel.Title,
		}

		// TODO: No peer.Chat.Username, how to get group chat username (for a public group)

		return v, peer.Chat.ID
	}

	return &proto.FLO_SOURCE{
		Flags: int32(proto.FLAGS_Invalid),
	}, -1
}

func (c *converter) makeProtoMessage(msg *tg.Message, source *proto.FLO_SOURCE, deepFromId int64) *proto.FLO_MESSAGE {

	messageDeepLinks := []string{
		fmt.Sprintf("https://t.me/c/%d/%d", deepFromId, msg.ID),
	}

	// TODO: proto add detection for username
	// if source.Flags&int32(proto.FLAGS_TgUsername) != 0 {
	// 	s := fmt.Sprintf("https://t.me/%s/%d", source.Username, msg.ID)
	// 	messageDeepLinks = append(messageDeepLinks, s)
	// }

	return &proto.FLO_MESSAGE{
		Flags:        source.Flags,
		CreatedAt:    timestamppb.New(time.Unix(int64(msg.Date), 0)),
		Title:        source.Title,
		SourceUid:    source.SourceUid,
		MessageUid:   fmt.Sprintf("%s-%d", source.SourceUid, msg.ID),
		Text:         msg.Message,
		MessageLinks: messageDeepLinks,
	}
}

func (c *converter) encodeToJson(m any, pretty bool) string {

	var (
		data []byte
		err  error
	)

	if pretty {
		data, err = json.MarshalIndent(m, "", "    ")
	} else {
		data, err = json.Marshal(m)
	}

	if err != nil {
		logInfo := map[string]interface{}{
			"err":        err.Error(),
			"prettty":    pretty,
			"debug_type": reflect.TypeOf(m),
		}
		c.bootstrap.Logger.Message(gelf.LOG_ERR, "converter", "encodeToJson failed to marshal object as JSON string", logInfo)

		return ""
	}

	return string(data)
}

func (c *converter) encodeRpcToBytes(m protobuf.Message) []byte {

	rpcbytes, err := protobuf.Marshal(m)
	if err != nil {
		logInfo := map[string]interface{}{
			"err":       err.Error(),
			"debug_rpc": c.encodeToJson(m, true),
		}
		c.bootstrap.Logger.Message(gelf.LOG_ERR, "converter", "encodeRpcToBytes failed to marshal protobuf message as binary", logInfo)

		return nil
	}

	return rpcbytes
}
