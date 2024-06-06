package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/go-faster/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type storedSource struct {
	ID        string             `bson:"_id"`
	CreatedAt primitive.DateTime `bson:"created_at"`
	Source    *proto.FLO_SOURCE  `bson:"source"`
	SourceRPC primitive.Binary   `bson:"source_rpc"`
}

type storedMessage struct {
	ID               string             `bson:"_id"`
	CreatedAt        primitive.DateTime `bson:"created_at"`
	MessageCreatedAt primitive.DateTime `bson:"message_created_at"`
	Message          *proto.FLO_MESSAGE `bson:"message"`
	MessageRPC       primitive.Binary   `bson:"message_rpc"`
}

type storageSave struct {
	storage *Storage
	logger  Logging
}

func (op *storageSave) Source(ctx context.Context, c *converter, source *proto.FLO_SOURCE) (StorageObjectID, error) {
	storage := op.storage

	db := storage.mgClient.Database(storage.dbName)

	err := op.EnsureCollection(ctx, db_collection_sources, "created_at")
	if err != nil {
		return "", errors.Wrapf(err, "EnsureCollection failed for %s", db_collection_sources)
	}

	col := db.Collection(db_collection_sources)

	m := storedSource{
		ID:        source.SourceUid,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		Source:    source,
		SourceRPC: primitive.Binary{
			Subtype: STORAGE_BINARY_RPC_SUBTYPE,
			Data:    c.encodeRpcToBytes(source),
		},
	}

	res, err := col.InsertOne(ctx, &m)
	if err != nil {
		op.logger.Message(gelf.LOG_ERR, "storage_save", "InsertOne failed (Source)", map[string]any{
			"col_name":   db_collection_sources,
			"err":        err,
			"debug_json": c.encodeToJson(m, true),
		})
		return "", errors.Wrap(err, "InsertOne failed (Source)")
	}

	op.logger.Message(gelf.LOG_INFO, "storage", fmt.Sprintf("InsertOne OK for Source %s", res.InsertedID), map[string]any{
		"col_name": db_collection_sources,
		"err":      err,
	})

	return StorageObjectID(res.InsertedID.(string)), err
}

func (op *storageSave) Message(ctx context.Context, c *converter, source *proto.FLO_SOURCE, message *proto.FLO_MESSAGE) (StorageObjectID, error) {
	storage := op.storage

	db := storage.mgClient.Database(storage.dbName)

	colName := strings.Trim(source.SourceUid, "- ")
	err := op.EnsureCollection(ctx, colName, "message_created_at")
	if err != nil {
		return "", errors.Wrapf(err, "EnsureCollection failed for %s", colName)
	}

	col := db.Collection(colName)

	m := storedMessage{
		ID:               message.MessageUid,
		CreatedAt:        primitive.NewDateTimeFromTime(time.Now().UTC()),
		MessageCreatedAt: primitive.NewDateTimeFromTime(message.CreatedAt.AsTime()),
		Message:          message,
		MessageRPC: primitive.Binary{
			Subtype: STORAGE_BINARY_RPC_SUBTYPE,
			Data:    c.encodeRpcToBytes(message),
		},
	}

	res, err := col.InsertOne(ctx, &m)
	if err != nil {
		op.logger.Message(gelf.LOG_ERR, "storage_save", "InsertOne failed (Message)", map[string]any{
			"col_name":   colName,
			"err":        err,
			"debug_json": c.encodeToJson(m, true),
		})
		return "", errors.Wrap(err, "InsertOne failed (Message)")
	}

	op.logger.Message(gelf.LOG_INFO, "storage", fmt.Sprintf("InsertOne OK for Message %s", res.InsertedID), map[string]any{
		"col_name": colName,
		"err":      err,
	})

	return StorageObjectID(res.InsertedID.(string)), err
}

func (op *storageSave) EnsureCollection(ctx context.Context, colName, timeField string) error {
	storage := op.storage

	db := storage.mgClient.Database(storage.dbName)

	names, err := db.ListCollectionNames(ctx, bson.D{}, nil) // TODO: speed up this query to avoid iterating all collections
	if err != nil {
		return errors.Wrap(err, "Failed to ListCollectionNames")
	}

	for _, name := range names {
		if name == colName {
			op.logger.Message(gelf.LOG_DEBUG, "storage_save", "Collection exists already", map[string]any{"col_name": colName})
			return nil
		}
	}
	// Timeseries collections must be explicitly created so we explicitly create it here
	opts := options.CreateCollection().
		SetTimeSeriesOptions(options.TimeSeries().
			SetGranularity("hours").
			SetMetaField("metadata").
			SetTimeField(timeField))
	err = db.CreateCollection(ctx, colName, opts)
	if err != nil {
		op.logger.Message(gelf.LOG_NOTICE, "storage_save", "Failed to created timeseries collection", map[string]any{
			"col_name":       colName,
			"col_time_field": timeField,
			"err":            err,
		})
		return errors.Wrap(err, "Error creating collection")
	}

	op.logger.Message(gelf.LOG_NOTICE, "storage_save", "Collection created as timeseries", map[string]any{
		"col_name":       colName,
		"col_time_field": timeField,
	})

	return nil
}
