package database

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type PaginatedScanResult struct {
	Items            []map[string]types.AttributeValue
	LastEvaluatedKey map[string]types.AttributeValue
	HasMore          bool
}

func attrString(value string) types.AttributeValue {
	return &types.AttributeValueMemberS{Value: value}
}

func (c *DynamoDBClient) PutItem(
	ctx context.Context,
	tableName string,
	item interface{},
) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal item: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	}

	_, err = c.svc.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("put item %s: %w", tableName, err)
	}
	return nil
}

func (c *DynamoDBClient) GetItem(
	ctx context.Context,
	tableName string,
	key map[string]types.AttributeValue,
	out interface{},
) error {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	}

	res, err := c.svc.GetItem(ctx, input)
	if err != nil {
		return fmt.Errorf("get item %s: %w", tableName, err)
	}
	if res.Item == nil {
		return fmt.Errorf("item not found in %s", tableName)
	}

	if err := attributevalue.UnmarshalMap(res.Item, out); err != nil {
		return fmt.Errorf("unmarshal item: %w", err)
	}
	return nil
}

func (c *DynamoDBClient) UpdateItem(
	ctx context.Context,
	tableName string,
	key map[string]types.AttributeValue,
	updateExpr string,
	exprAttrValues map[string]types.AttributeValue,
	exprAttrNames map[string]string,
	out interface{},
) error {
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(tableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
		ExpressionAttributeNames:  exprAttrNames,
		ReturnValues:              types.ReturnValueAllNew,
	}

	res, err := c.svc.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("update item %s: %w", tableName, err)
	}

	if out != nil {
		if err := attributevalue.UnmarshalMap(res.Attributes, out); err != nil {
			return fmt.Errorf("unmarshal updated item: %w", err)
		}
	}
	return nil
}

func (c *DynamoDBClient) DeleteItem(
	ctx context.Context,
	tableName string,
	key map[string]types.AttributeValue,
) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	}

	_, err := c.svc.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("delete item %s: %w", tableName, err)
	}
	return nil
}

func (c *DynamoDBClient) QueryItems(
	ctx context.Context,
	tableName string,
	indexName *string,
	keyCondExpr string,
	exprAttrValues map[string]types.AttributeValue,
	exprAttrNames map[string]string,
	scanIndexForward *bool,
) ([]map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		KeyConditionExpression:    aws.String(keyCondExpr),
		ExpressionAttributeValues: exprAttrValues,
	}
	if indexName != nil {
		input.IndexName = indexName
	}
	if exprAttrNames != nil {
		input.ExpressionAttributeNames = exprAttrNames
	}

	if scanIndexForward != nil {
		input.ScanIndexForward = aws.Bool(*scanIndexForward)
	}

	out, err := c.svc.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("query %s[%s]: %w", tableName, aws.ToString(indexName), err)
	}

	return out.Items, nil
}

func (c *DynamoDBClient) QueryItemsWithFilter(
	ctx context.Context,
	tableName string,
	indexName *string,
	keyCondExpr string,
	filterExpr *string,
	exprAttrValues map[string]types.AttributeValue,
	exprAttrNames map[string]string,
) ([]map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		KeyConditionExpression:    aws.String(keyCondExpr),
		ExpressionAttributeValues: exprAttrValues,
	}
	if indexName != nil {
		input.IndexName = indexName
	}
	if filterExpr != nil {
		input.FilterExpression = filterExpr
	}
	if exprAttrNames != nil {
		input.ExpressionAttributeNames = exprAttrNames
	}
	out, err := c.svc.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("query %s[%s]: %w", tableName, aws.ToString(indexName), err)
	}
	return out.Items, nil
}

func (c *DynamoDBClient) ScanItems(
	ctx context.Context,
	tableName string,
	filterExpr string,
	exprAttrValues map[string]types.AttributeValue,
	exprAttrNames map[string]string, // Add this parameter
) ([]map[string]types.AttributeValue, error) {
	input := &dynamodb.ScanInput{
		TableName:                 aws.String(tableName),
		FilterExpression:          aws.String(filterExpr),
		ExpressionAttributeValues: exprAttrValues,
	}

	if exprAttrNames != nil {
		input.ExpressionAttributeNames = exprAttrNames
	}

	out, err := c.svc.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", tableName, err)
	}

	return out.Items, nil
}

func (c *DynamoDBClient) BatchWriteItems(
	ctx context.Context,
	requests map[string][]types.WriteRequest,
) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: requests,
	}

	_, err := c.svc.BatchWriteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("batch write: %w", err)
	}
	return nil
}

func (c *DynamoDBClient) BatchGetItems(
	ctx context.Context,
	requestItems map[string]types.KeysAndAttributes,
) (map[string][]map[string]types.AttributeValue, error) {
	input := &dynamodb.BatchGetItemInput{
		RequestItems: requestItems,
	}

	res, err := c.svc.BatchGetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("batch get: %w", err)
	}
	return res.Responses, nil
}

func (c *DynamoDBClient) BatchGetByKeys(
	ctx context.Context,
	tableName string,
	keyValues []string,
	keyField string,
	batchSize int,
	indexName *string, // Optional GSI name
) ([]map[string]types.AttributeValue, error) {
	if len(keyValues) == 0 {
		return []map[string]types.AttributeValue{}, nil
	}

	if batchSize <= 0 || batchSize > 100 {
		batchSize = 100
	}

	var allItems []map[string]types.AttributeValue

	for i := 0; i < len(keyValues); i += batchSize {
		end := i + batchSize
		if end > len(keyValues) {
			end = len(keyValues)
		}

		batchValues := keyValues[i:end]

		var items []map[string]types.AttributeValue
		var err error

		if indexName != nil {
			items, err = c.batchQueryGSIChunk(ctx, tableName, *indexName, batchValues, keyField)
		} else {
			items, err = c.batchGetChunk(ctx, tableName, batchValues, keyField)
		}

		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)
	}

	return allItems, nil
}

func (c *DynamoDBClient) batchQueryGSIChunk(
	ctx context.Context,
	tableName string,
	indexName string,
	keyValues []string,
	keyField string,
) ([]map[string]types.AttributeValue, error) {
	var allItems []map[string]types.AttributeValue

	for _, keyValue := range keyValues {
		keyCondExpr := fmt.Sprintf("%s = :keyval", keyField)
		exprAttrValues := map[string]types.AttributeValue{
			":keyval": attrString(keyValue),
		}

		items, err := c.QueryItems(ctx, tableName, &indexName, keyCondExpr, exprAttrValues, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to query GSI %s for key %s: %w", indexName, keyValue, err)
		}

		allItems = append(allItems, items...)
	}

	return allItems, nil
}

func (c *DynamoDBClient) batchGetChunk(
	ctx context.Context,
	tableName string,
	keyValues []string,
	keyField string,
) ([]map[string]types.AttributeValue, error) {
	keys := make([]map[string]types.AttributeValue, len(keyValues))
	for i, value := range keyValues {
		keys[i] = map[string]types.AttributeValue{
			keyField: attrString(value),
		}
	}

	requestItems := map[string]types.KeysAndAttributes{
		tableName: {
			Keys: keys,
		},
	}

	responses, err := c.BatchGetItems(ctx, requestItems)
	if err != nil {
		return nil, err
	}

	items, exists := responses[tableName]
	if !exists {
		return []map[string]types.AttributeValue{}, nil
	}

	return items, nil
}

// ScanPaginated performs a paginated scan operation
func (c *DynamoDBClient) ScanPaginated(
	ctx context.Context,
	tableName string,
	pageSize int,
	lastEvaluatedKey map[string]types.AttributeValue,
) (*PaginatedScanResult, error) {
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(int32(pageSize)),
	}

	if lastEvaluatedKey != nil {
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	result, err := c.svc.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("scan paginated %s: %w", tableName, err)
	}

	return &PaginatedScanResult{
		Items:            result.Items,
		LastEvaluatedKey: result.LastEvaluatedKey,
		HasMore:          result.LastEvaluatedKey != nil,
	}, nil
}

// ScanAll performs a complete scan of the table, handling pagination internally
func (c *DynamoDBClient) ScanAll(
	ctx context.Context,
	tableName string,
) ([]map[string]types.AttributeValue, error) {
	var allItems []map[string]types.AttributeValue
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
		}

		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := c.svc.Scan(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("scan all %s: %w", tableName, err)
		}

		allItems = append(allItems, result.Items...)

		if result.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	return allItems, nil
}

func (c *DynamoDBClient) QueryPaginated(
	ctx context.Context,
	tableName string,
	indexName *string,
	keyCondExpr string,
	exprAttrValues map[string]types.AttributeValue,
	pageSize int,
	lastEvaluatedKey map[string]types.AttributeValue,
	scanIndexForward *bool,
) (*PaginatedScanResult, error) {
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		KeyConditionExpression:    aws.String(keyCondExpr),
		ExpressionAttributeValues: exprAttrValues,
		Limit:                     aws.Int32(int32(pageSize)),
	}

	if indexName != nil {
		input.IndexName = indexName
	}

	if lastEvaluatedKey != nil {
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	if scanIndexForward != nil {
		input.ScanIndexForward = aws.Bool(*scanIndexForward)
	}

	result, err := c.svc.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("query paginated %s[%s]: %w", tableName, aws.ToString(indexName), err)
	}

	return &PaginatedScanResult{
		Items:            result.Items,
		LastEvaluatedKey: result.LastEvaluatedKey,
		HasMore:          result.LastEvaluatedKey != nil,
	}, nil
}

// QueryAll performs a complete query, handling pagination internally.
func (c *DynamoDBClient) QueryAll(
	ctx context.Context,
	tableName string,
	indexName *string,
	keyCondExpr string,
	exprAttrValues map[string]types.AttributeValue,
) ([]map[string]types.AttributeValue, error) {
	var allItems []map[string]types.AttributeValue
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		input := &dynamodb.QueryInput{
			TableName:                 aws.String(tableName),
			KeyConditionExpression:    aws.String(keyCondExpr),
			ExpressionAttributeValues: exprAttrValues,
		}

		if indexName != nil {
			input.IndexName = indexName
		}

		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := c.svc.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("query all %s[%s]: %w", tableName, aws.ToString(indexName), err)
		}

		allItems = append(allItems, result.Items...)

		if result.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	return allItems, nil
}

func (c *DynamoDBClient) ScanAllWithFilter(
	ctx context.Context,
	tableName string,
	filterExpr string,
	exprAttrValues map[string]types.AttributeValue,
	exprAttrNames map[string]string,
) ([]map[string]types.AttributeValue, error) {
	var allItems []map[string]types.AttributeValue
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:                 aws.String(tableName),
			FilterExpression:          aws.String(filterExpr),
			ExpressionAttributeValues: exprAttrValues,
		}
		if exprAttrNames != nil {
			input.ExpressionAttributeNames = exprAttrNames
		}
		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := c.svc.Scan(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("scan all with filter %s: %w", tableName, err)
		}

		allItems = append(allItems, result.Items...)

		if result.LastEvaluatedKey == nil || len(result.LastEvaluatedKey) == 0 {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}
	return allItems, nil
}

func (c *DynamoDBClient) BatchWriteItem(
	ctx context.Context,
	tableName string,
	putItems []interface{},
	deleteKeys []map[string]types.AttributeValue,
) error {
	if len(putItems) == 0 && len(deleteKeys) == 0 {
		return nil
	}

	var writeRequests []types.WriteRequest

	for _, item := range putItems {
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("marshal put item: %w", err)
		}

		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: av,
			},
		})
	}

	for _, key := range deleteKeys {
		writeRequests = append(writeRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: key,
			},
		})
	}

	const batchSize = 25
	for i := 0; i < len(writeRequests); i += batchSize {
		end := i + batchSize
		if end > len(writeRequests) {
			end = len(writeRequests)
		}

		batch := writeRequests[i:end]
		requests := map[string][]types.WriteRequest{
			tableName: batch,
		}

		if err := c.batchWriteWithRetry(ctx, tableName, requests); err != nil {
			return fmt.Errorf("batch write item: %w", err)
		}
	}

	return nil
}

func (c *DynamoDBClient) batchWriteWithRetry(
	ctx context.Context,
	tableName string,
	requests map[string][]types.WriteRequest,
) error {
	const maxRetries = 3
	retryCount := 0
	currentRequests := requests

	for len(currentRequests) > 0 && retryCount < maxRetries {
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: currentRequests,
		}

		result, err := c.svc.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("batch write (attempt %d): %w", retryCount+1, err)
		}

		if len(result.UnprocessedItems) == 0 {
			break
		}

		currentRequests = result.UnprocessedItems
		retryCount++

		if retryCount < maxRetries {
			backoffDuration := time.Duration(1<<uint(retryCount-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDuration):
			}
		}
	}

	if len(currentRequests) > 0 {
		unprocessedKeys := extractKeysFromUnprocessed(currentRequests)

		allDeleted := true
		for _, key := range unprocessedKeys {
			if key == nil {
				allDeleted = false
				break
			}
			getInput := &dynamodb.GetItemInput{
				TableName: aws.String(tableName),
				Key:       key,
			}
			result, err := c.svc.GetItem(ctx, getInput)
			if err != nil {
				allDeleted = false
				break
			}
			if len(result.Item) > 0 {
				allDeleted = false
				break
			}
		}

		if allDeleted {
			return nil
		}

		return fmt.Errorf(
			"failed to process all items after %d retries, %d items remain unprocessed; keys: %v",
			maxRetries, countUnprocessedItems(currentRequests), unprocessedKeys,
		)
	}

	return nil
}

func extractKeysFromUnprocessed(requests map[string][]types.WriteRequest) []map[string]types.AttributeValue {
	var keys []map[string]types.AttributeValue
	for _, reqs := range requests {
		for _, req := range reqs {
			if req.DeleteRequest != nil && req.DeleteRequest.Key != nil {
				keys = append(keys, req.DeleteRequest.Key)
			}
		}
	}
	return keys
}

func countUnprocessedItems(requests map[string][]types.WriteRequest) int {
	count := 0
	for _, reqs := range requests {
		count += len(reqs)
	}
	return count
}

func (c *DynamoDBClient) BatchDeleteItems(
	ctx context.Context,
	tableName string,
	keys []map[string]types.AttributeValue,
) error {
	return c.BatchWriteItem(ctx, tableName, nil, keys)
}

func (c *DynamoDBClient) ParallelScanWithFilter(
	ctx context.Context,
	tableName string,
	filterExpr string,
	exprAttrValues map[string]types.AttributeValue,
	exprAttrNames map[string]string,
	projectionExpr *string,
	totalSegments int,
) ([]map[string]types.AttributeValue, error) {
	if totalSegments < 1 {
		totalSegments = 8
	} // tune to vCPU
	type segRes struct {
		items []map[string]types.AttributeValue
		err   error
	}
	out := make(chan segRes, totalSegments)

	for seg := 0; seg < totalSegments; seg++ {
		seg := seg
		go func() {
			var all []map[string]types.AttributeValue
			var last map[string]types.AttributeValue
			for {
				in := &dynamodb.ScanInput{
					TableName:                 aws.String(tableName),
					FilterExpression:          aws.String(filterExpr),
					ExpressionAttributeValues: exprAttrValues,
					Segment:                   aws.Int32(int32(seg)),
					TotalSegments:             aws.Int32(int32(totalSegments)),
				}
				if exprAttrNames != nil {
					in.ExpressionAttributeNames = exprAttrNames
				}
				if projectionExpr != nil {
					in.ProjectionExpression = projectionExpr
				}
				if last != nil {
					in.ExclusiveStartKey = last
				}

				res, err := c.svc.Scan(ctx, in)
				if err != nil {
					out <- segRes{nil, fmt.Errorf("scan seg %d: %w", seg, err)}
					return
				}

				all = append(all, res.Items...)
				if len(res.LastEvaluatedKey) == 0 {
					break
				}
				last = res.LastEvaluatedKey
			}
			out <- segRes{all, nil}
		}()
	}

	var all []map[string]types.AttributeValue
	for i := 0; i < totalSegments; i++ {
		r := <-out
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.items...)
	}
	return all, nil
}
