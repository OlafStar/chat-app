package database

import (
	"chat-app-backend/internal/env"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type DynamoDBClient struct {
	svc *dynamodb.Client
}

func NewDynamoDBClient() (*DynamoDBClient, error) {
	region := env.Get(env.AWSRegion)
	credOne := env.Get(env.AWSID)
	credTwo := env.Get(env.AWSSecret)
	credThree := env.Get(env.AWSToken)
	endpoint := env.Get(env.DynamoDBEndpoint)

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	if credOne != "" && credTwo != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(credOne, credTwo, credThree)),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	clientOpts := []func(*dynamodb.Options){}
	if endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	db := dynamodb.NewFromConfig(cfg, clientOpts...)
	return &DynamoDBClient{
		svc: db,
	}, nil
}

type Database struct {
	Client *DynamoDBClient
}

func NewDatabase() (*Database, error) {
	dbClient, err := NewDynamoDBClient()
	if err != nil {
		return nil, fmt.Errorf("init dynamodb client: %w", err)
	}

	return &Database{
		Client: dbClient,
	}, nil
}
