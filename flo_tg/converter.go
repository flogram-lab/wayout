package main

import (
	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/gotd/contrib/storage"
	"github.com/gotd/td/tg"
)

type converter struct {
	bootstrap Bootstrap
}

func newConverter(bootstrap Bootstrap) *converter {
	return &converter{
		bootstrap: bootstrap,
	}
}

func (self *converter) makeSource(msg *tg.Message, peer storage.Peer, e tg.Entities) *proto.FLO_SOURCE {
	// TODO
	return nil
}

func (self *converter) makeMessage(msg *tg.Message, source *proto.FLO_SOURCE) *proto.FLO_MESSAGE {
	// TODO
	return nil
}
