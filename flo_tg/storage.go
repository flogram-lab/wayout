package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StorageObjectID string

const (
	db_collection_sources           = "tgv1-sources"
	STORAGE_BINARY_RPC_SUBTYPE byte = 255 // 0xff
)

type Storage struct {
	mgClient *mongo.Client
	dbName   string
}

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

func NewStorageMongo(uri string, databaseName string) *Storage {

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
		log.Println("ERROR Close() mmongoogno connection", err)
	}
}

func (storage *Storage) StoreSource(ctx context.Context, c *converter, source *proto.FLO_SOURCE) (StorageObjectID, error) {

	db := storage.mgClient.Database(storage.dbName)

	err := storage.ensureCollection(ctx, db_collection_sources, "created_at")
	if err != nil {
		return "", errors.Wrapf(err, "ensureCollection failed for %s", db_collection_sources)
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
		return "", errors.Wrap(err, "InsertOne failed (StoreSource)")
	}

	return StorageObjectID(res.InsertedID.(string)), err
}

func (storage *Storage) StoreMessage(ctx context.Context, c *converter, source *proto.FLO_SOURCE, message *proto.FLO_MESSAGE) (StorageObjectID, error) {

	db := storage.mgClient.Database(storage.dbName)

	colName := strings.Trim(source.SourceUid, "- ")
	err := storage.ensureCollection(ctx, colName, "message_created_at")
	if err != nil {
		return "", errors.Wrapf(err, "ensureCollection failed for %s", colName)
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
		return "", errors.Wrap(err, "InsertOne failed (StoreMessage)")
	}

	return StorageObjectID(res.InsertedID.(string)), err
}

func (storage *Storage) ensureCollection(ctx context.Context, colName, timeField string) error {

	db := storage.mgClient.Database(storage.dbName)

	exists := false
	names, err := db.ListCollectionNames(ctx, bson.D{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to ListCollectionNames")
	}

	for _, name := range names {
		if name == colName {
			exists = true
			log.Printf("%s table already exists. continuing.", colName)
		}
	}

	if !exists {
		// Timeseries collections must be explicitly created so we explicitly create it here
		opts := options.CreateCollection().
			SetTimeSeriesOptions(options.TimeSeries().
				SetGranularity("hours").
				SetMetaField("metadata").
				SetTimeField(timeField))
		err = db.CreateCollection(ctx, colName, opts)
		if err != nil {
			return errors.Wrap(err, "Error creating collection")
		} else {
			log.Printf("Successfully created %s table for the first time.", colName)
		}
	}

	return nil
}
