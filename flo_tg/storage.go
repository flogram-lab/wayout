package main

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type StorageObjectID string

const (
	db_collection_sources           = "tgv1-sources"
	STORAGE_BINARY_RPC_SUBTYPE byte = 255 // 0xff
)

type Storage struct {
	logger   Logger
	mgClient *mongo.Client
	dbName   string
}

func NewStorageMongo(uri string, databaseName string, logger Logger) *Storage {

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	return &Storage{
		mgClient: client,
		dbName:   databaseName,
	}
}

func (storage *Storage) Ping() error {
	result := &bson.M{}
	return storage.mgClient.Database(storage.dbName).RunCommand(context.TODO(), bson.D{{"ping", 1}}).Decode(&result)
}

func (storage *Storage) Close() {
	if err := storage.mgClient.Disconnect(context.TODO()); err != nil {
		storage.logger.Message(gelf.LOG_WARNING, "storage", "ERROR Close() mongodb connection", map[string]any{
			"err": err,
		})
	}
}
