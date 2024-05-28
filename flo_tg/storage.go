package main

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Storage struct {
	mgClient *mongo.Client
	dbName   string
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
