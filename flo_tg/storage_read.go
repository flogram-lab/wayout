package main

import (
	"context"
	"strings"

	"github.com/golang/protobuf/proto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type storageRead struct {
	storage *Storage
	logger  Logger
}

// TODO: streaming. use channel, and support context cancellation?
func (op *storageRead) Sources(ctx context.Context, uids ...string) ([]storedSource, error) {
	storage := op.storage

	db := storage.mgClient.Database(storage.dbName)

	col := db.Collection(db_collection_sources)

	filter := bson.D{}

	if len(uids) > 0 {
		filter = bson.D{{"_id", bson.D{{"$in", uids}}}}
	}

	cur, err := col.Find(ctx, filter)
	if err != nil {
		op.logger.Message(gelf.LOG_ERR, "storage_read", "Find documents failed (sources)", map[string]any{
			"col_name":   db_collection_sources,
			"debug_uids": strings.Join(uids, ","),
			"err":        err,
		})
		return nil, err
	}

	result := []storedSource{}

	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var m storedSource
		err := cur.Decode(&m)

		if err != nil {
			op.logger.Message(gelf.LOG_ERR, "storage_read", "Decode failed for DB document (skipped)", map[string]any{
				"col_name": db_collection_sources,
				"err":      err,
				"id":       cur.ID(),
			})
		}

		if err := proto.Unmarshal(m.SourceRPC.Data, m.Source); err != nil {
			op.logger.Message(gelf.LOG_ERR, "storage_read", "Unmarshal RPC failed (skipped)", map[string]any{
				"col_name": db_collection_sources,
				"id":       cur.ID(),
				"err":      err,
			})

			continue
		}

		result = append(result, m)

	}

	if err := cur.Err(); err != nil {
		op.logger.Message(gelf.LOG_ERR, "storage_read", "Closing find cursor with error", map[string]any{
			"col_name": db_collection_sources,
			"err":      err,
		})
		return nil, err
	}

	return result, nil
}

// TODO: streaming. use channel, and support context cancellation?
func (op *storageRead) Messages(ctx context.Context, sourceUid string) ([]storedMessage, error) {
	storage := op.storage

	db := storage.mgClient.Database(storage.dbName)

	col := db.Collection(sourceUid)

	filter := bson.D{}

	opts := options.Find().SetSort(bson.D{{"message_created_at", 1}})

	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		op.logger.Message(gelf.LOG_ERR, "storage_read", "Find documents failed (messages by source)", map[string]any{
			"col_name": sourceUid,
			"err":      err,
		})
		return nil, err
	}

	result := []storedMessage{}

	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var m storedMessage
		err := cur.Decode(&m)

		if err != nil {
			op.logger.Message(gelf.LOG_ERR, "storage_read", "Decode failed for DB document (skipped)", map[string]any{
				"col_name": sourceUid,
				"err":      err,
				"id":       cur.ID(),
			})
		}

		if err := proto.Unmarshal(m.MessageRPC.Data, m.Message); err != nil {
			op.logger.Message(gelf.LOG_ERR, "storage_read", "Unmarshal RPC failed (skipped)", map[string]any{
				"col_name": db_collection_sources,
				"id":       cur.ID(),
				"err":      err,
			})

			continue
		}

		result = append(result, m)

	}

	if err := cur.Err(); err != nil {
		op.logger.Message(gelf.LOG_ERR, "storage_read", "Closing find cursor with error", map[string]any{
			"col_name": sourceUid,
			"err":      err,
		})
		return nil, err
	}

	return result, err
}
