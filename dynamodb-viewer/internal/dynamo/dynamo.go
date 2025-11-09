package dynamo

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Config captures the minimum settings required to talk to DynamoDB.
type Config struct {
	Region       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	SessionToken string
}

// Service provides higher level helpers around the DynamoDB SDK.
type Service struct {
	client *dynamodb.Client
}

// New creates a Service backed by a DynamoDB client.
func New(ctx context.Context, cfg Config) (*Service, error) {
	if cfg.Region == "" {
		return nil, errors.New("region is required")
	}

	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken)

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(creds),
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	clientOpts := []func(*dynamodb.Options){}
	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	client := dynamodb.NewFromConfig(awsCfg, clientOpts...)

	return &Service{client: client}, nil
}

// Client exposes the raw DynamoDB client for advanced scenarios.
func (s *Service) Client() *dynamodb.Client {
	return s.client
}

// ListTables returns all table names in the account/endpoint.
func (s *Service) ListTables(ctx context.Context) ([]string, error) {
	var last *string
	var names []string

	for {
		out, err := s.client.ListTables(ctx, &dynamodb.ListTablesInput{
			ExclusiveStartTableName: last,
			Limit:                   aws.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}

		names = append(names, out.TableNames...)
		if out.LastEvaluatedTableName == nil {
			break
		}
		last = out.LastEvaluatedTableName
	}

	return names, nil
}

// DescribeTable returns metadata for a single table.
func (s *Service) DescribeTable(ctx context.Context, table string) (*types.TableDescription, error) {
	out, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)})
	if err != nil {
		return nil, fmt.Errorf("describe table %s: %w", table, err)
	}
	return out.Table, nil
}

// ScanResult contains a slice of hydrated items plus pagination metadata.
type ScanResult struct {
	Items            []map[string]interface{} `json:"items"`
	LastEvaluatedKey map[string]interface{}   `json:"lastEvaluatedKey,omitempty"`
	Count            int32                    `json:"count"`
	ScannedCount     int32                    `json:"scannedCount"`
}

// ScanTable performs a scan with pagination support.
func (s *Service) ScanTable(ctx context.Context, table string, limit int32, startKey map[string]types.AttributeValue) (*ScanResult, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(table),
		Limit:     aws.Int32(limit),
	}

	if startKey != nil {
		input.ExclusiveStartKey = startKey
	}

	out, err := s.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("scan table %s: %w", table, err)
	}

	items, err := attributevalueToMaps(out.Items)
	if err != nil {
		return nil, err
	}

	lastKey, err := attributevalueMapToInterface(out.LastEvaluatedKey)
	if err != nil {
		return nil, err
	}

	return &ScanResult{
		Items:            items,
		LastEvaluatedKey: lastKey,
		Count:            out.Count,
		ScannedCount:     out.ScannedCount,
	}, nil
}

func attributevalueToMaps(items []map[string]types.AttributeValue) ([]map[string]interface{}, error) {
	if len(items) == 0 {
		return make([]map[string]interface{}, 0), nil
	}

	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		var decoded map[string]interface{}
		if err := attributevalue.UnmarshalMap(item, &decoded); err != nil {
			return nil, fmt.Errorf("decode item: %w", err)
		}
		result = append(result, decoded)
	}
	return result, nil
}

func attributevalueMapToInterface(av map[string]types.AttributeValue) (map[string]interface{}, error) {
	if av == nil {
		return nil, nil
	}
	var decoded map[string]interface{}
	if err := attributevalue.UnmarshalMap(av, &decoded); err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	return decoded, nil
}

// MarshalInterfaceMap converts a loose map into DynamoDB's attribute representation.
func MarshalInterfaceMap(input map[string]interface{}) (map[string]types.AttributeValue, error) {
	if input == nil {
		return nil, nil
	}
	marshalled, err := attributevalue.MarshalMap(input)
	if err != nil {
		return nil, fmt.Errorf("encode key: %w", err)
	}
	return marshalled, nil
}
