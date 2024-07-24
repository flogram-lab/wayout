package main

import (
	"github.com/flogram-lab/wayout/flo_tg/proto"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type storedSource struct {
	ID        string             `bson:"_id"`
	CreatedAt primitive.DateTime `bson:"created_at"`
	Source    *proto.FLO_SOURCE  `bson:"source"`
	SourceRPC primitive.Binary   `bson:"source_rpc"`
	//CanonicalTitle string TODO: track sources Title changes
}

type storedMessage struct {
	ID               string             `bson:"_id"`
	CreatedAt        primitive.DateTime `bson:"created_at"`
	MessageCreatedAt primitive.DateTime `bson:"message_created_at"`
	Message          *proto.FLO_MESSAGE `bson:"message"`
	MessageRPC       primitive.Binary   `bson:"message_rpc"`
}
