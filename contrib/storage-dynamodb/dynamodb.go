// Package dynamodb provides AWS DynamoDB-backed implementations of agent-go storage interfaces.
//
// DynamoDB is a fully managed NoSQL database service that provides fast and predictable
// performance with seamless scalability. It is ideal for serverless architectures and
// applications requiring consistent, single-digit millisecond latency at any scale.
//
// # Usage
//
//	cfg, err := config.LoadDefaultConfig(context.Background())
//	if err != nil {
//		return err
//	}
//
//	client := dynamodb.NewFromConfig(cfg)
//	cache := storagedynamodb.NewCache(client, "agent-cache-table")
//	runStore := storagedynamodb.NewRunStore(client, "agent-runs-table")
package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/run"
)

// dynamoDBAPI defines the DynamoDB operations we use.
// This allows for mocking in tests.
type dynamoDBAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

// Client represents a DynamoDB client interface.
// This allows for mocking in tests.
type Client = dynamoDBAPI

// Cache is a DynamoDB-backed implementation of cache.Cache.
// It stores cached values in a DynamoDB table with optional TTL support
// using DynamoDB's native TTL feature.
type Cache struct {
	client    Client
	tableName string
}

// CacheConfig holds configuration for the DynamoDB cache.
type CacheConfig struct {
	// TableName is the DynamoDB table name.
	TableName string

	// TTLAttributeName is the attribute name for TTL (default: "ttl").
	TTLAttributeName string

	// KeyPrefix is an optional prefix for all cache keys.
	KeyPrefix string
}

// NewCache creates a new DynamoDB cache with the given client and table name.
func NewCache(client Client, tableName string) *Cache {
	return &Cache{
		client:    client,
		tableName: tableName,
	}
}

// NewCacheWithConfig creates a new DynamoDB cache with full configuration.
func NewCacheWithConfig(client Client, cfg CacheConfig) *Cache {
	return &Cache{
		client:    client,
		tableName: cfg.TableName,
	}
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("context error: %w", err)
	}

	// Validate key
	if key == "" {
		return nil, false, cache.ErrInvalidKey
	}

	// Get item from DynamoDB
	output, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &c.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return nil, false, fmt.Errorf("dynamodb get failed: %w", err)
	}

	// Item not found
	if output.Item == nil {
		return nil, false, nil
	}

	// Check TTL expiration
	if ttlAttr, ok := output.Item["ttl"]; ok {
		if ttlNum, ok := ttlAttr.(*types.AttributeValueMemberN); ok {
			ttl, err := strconv.ParseInt(ttlNum.Value, 10, 64)
			if err == nil && ttl < time.Now().Unix() {
				// Expired
				return nil, false, nil
			}
		}
	}

	// Extract value
	valueAttr, ok := output.Item["value"]
	if !ok {
		return nil, false, fmt.Errorf("item missing value attribute")
	}

	valueBin, ok := valueAttr.(*types.AttributeValueMemberB)
	if !ok {
		return nil, false, fmt.Errorf("value attribute is not binary")
	}

	return valueBin.Value, true, nil
}

// Set stores a value with the given key and options.
// TTL is supported using DynamoDB's native TTL feature.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	// Validate key
	if key == "" {
		return cache.ErrInvalidKey
	}

	// Build item
	item := map[string]types.AttributeValue{
		"pk":    &types.AttributeValueMemberS{Value: key},
		"value": &types.AttributeValueMemberB{Value: value},
	}

	// Add TTL if specified
	if opts.TTL > 0 {
		expiryTime := time.Now().Add(opts.TTL).Unix()
		item["ttl"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(expiryTime, 10)}
	}

	// Put item
	_, err := c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &c.tableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb put failed: %w", err)
	}

	return nil
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	// Validate key
	if key == "" {
		return cache.ErrInvalidKey
	}

	// Delete item
	_, err := c.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &c.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb delete failed: %w", err)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	// Validate key
	if key == "" {
		return false, cache.ErrInvalidKey
	}

	// Get item with projection to minimize data transfer
	output, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &c.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
		ProjectionExpression: aws.String("pk, #ttl"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
	})
	if err != nil {
		return false, fmt.Errorf("dynamodb get failed: %w", err)
	}

	// Item not found
	if output.Item == nil {
		return false, nil
	}

	// Check TTL expiration
	if ttlAttr, ok := output.Item["ttl"]; ok {
		if ttlNum, ok := ttlAttr.(*types.AttributeValueMemberN); ok {
			ttl, err := strconv.ParseInt(ttlNum.Value, 10, 64)
			if err == nil && ttl < time.Now().Unix() {
				// Expired
				return false, nil
			}
		}
	}

	return true, nil
}

// Clear removes all entries from the cache.
// Note: DynamoDB does not support table truncation; this uses Scan + BatchWrite.
// For large tables, consider using a different approach or accepting eventual cleanup via TTL.
func (c *Cache) Clear(ctx context.Context) error {
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		// Scan for items
		scanInput := &dynamodb.ScanInput{
			TableName:            &c.tableName,
			ProjectionExpression: aws.String("pk"),
		}
		if lastEvaluatedKey != nil {
			scanInput.ExclusiveStartKey = lastEvaluatedKey
		}

		output, err := c.client.Scan(ctx, scanInput)
		if err != nil {
			return fmt.Errorf("dynamodb scan failed: %w", err)
		}

		// Delete items in batches of 25 (DynamoDB limit)
		if len(output.Items) > 0 {
			if err := c.deleteBatch(ctx, output.Items); err != nil {
				return err
			}
		}

		// Check for more items
		if output.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = output.LastEvaluatedKey
	}

	return nil
}

// deleteBatch deletes items in batches of 25.
func (c *Cache) deleteBatch(ctx context.Context, items []map[string]types.AttributeValue) error {
	const batchSize = 25

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		writeRequests := make([]types.WriteRequest, len(batch))

		for j, item := range batch {
			writeRequests[j] = types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: map[string]types.AttributeValue{
						"pk": item["pk"],
					},
				},
			}
		}

		_, err := c.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				c.tableName: writeRequests,
			},
		})
		if err != nil {
			return fmt.Errorf("dynamodb batch write failed: %w", err)
		}
	}

	return nil
}

// dynamoRun represents a Run stored in DynamoDB.
// We use this to control serialization format.
type dynamoRun struct {
	ID              string `dynamodbav:"id"`
	Goal            string `dynamodbav:"goal"`
	CurrentState    string `dynamodbav:"current_state"`
	Vars            string `dynamodbav:"vars,omitempty"`
	Evidence        string `dynamodbav:"evidence"`
	Status          string `dynamodbav:"status"`
	StartTime       string `dynamodbav:"start_time"`
	EndTime         string `dynamodbav:"end_time,omitempty"`
	Result          string `dynamodbav:"result,omitempty"`
	Error           string `dynamodbav:"error,omitempty"`
	PendingQuestion string `dynamodbav:"pending_question,omitempty"`
}

// runToDynamo converts an agent.Run to dynamoRun.
func runToDynamo(r *agent.Run) (*dynamoRun, error) {
	dr := &dynamoRun{
		ID:           r.ID,
		Goal:         r.Goal,
		CurrentState: string(r.CurrentState),
		Status:       string(r.Status),
		StartTime:    r.StartTime.Format(time.RFC3339Nano),
		Error:        r.Error,
	}

	// Marshal Vars
	if r.Vars != nil && len(r.Vars) > 0 {
		varsData, err := json.Marshal(r.Vars)
		if err != nil {
			return nil, fmt.Errorf("marshal vars: %w", err)
		}
		dr.Vars = string(varsData)
	}

	// Marshal Evidence
	if len(r.Evidence) > 0 {
		evidenceData, err := json.Marshal(r.Evidence)
		if err != nil {
			return nil, fmt.Errorf("marshal evidence: %w", err)
		}
		dr.Evidence = string(evidenceData)
	} else {
		dr.Evidence = "[]"
	}

	// EndTime
	if !r.EndTime.IsZero() {
		dr.EndTime = r.EndTime.Format(time.RFC3339Nano)
	}

	// Result
	if len(r.Result) > 0 {
		dr.Result = string(r.Result)
	}

	// PendingQuestion
	if r.PendingQuestion != nil {
		pqData, err := json.Marshal(r.PendingQuestion)
		if err != nil {
			return nil, fmt.Errorf("marshal pending question: %w", err)
		}
		dr.PendingQuestion = string(pqData)
	}

	return dr, nil
}

// dynamoToRun converts a dynamoRun to agent.Run.
func dynamoToRun(dr *dynamoRun) (*agent.Run, error) {
	r := &agent.Run{
		ID:           dr.ID,
		Goal:         dr.Goal,
		CurrentState: agent.State(dr.CurrentState),
		Status:       agent.RunStatus(dr.Status),
		Error:        dr.Error,
	}

	// Parse StartTime
	startTime, err := time.Parse(time.RFC3339Nano, dr.StartTime)
	if err != nil {
		return nil, fmt.Errorf("parse start_time: %w", err)
	}
	r.StartTime = startTime

	// Parse EndTime
	if dr.EndTime != "" {
		endTime, err := time.Parse(time.RFC3339Nano, dr.EndTime)
		if err != nil {
			return nil, fmt.Errorf("parse end_time: %w", err)
		}
		r.EndTime = endTime
	}

	// Unmarshal Vars
	if dr.Vars != "" {
		var vars map[string]any
		if err := json.Unmarshal([]byte(dr.Vars), &vars); err != nil {
			return nil, fmt.Errorf("unmarshal vars: %w", err)
		}
		r.Vars = vars
	} else {
		r.Vars = make(map[string]any)
	}

	// Unmarshal Evidence
	if dr.Evidence != "" && dr.Evidence != "[]" {
		var evidence []agent.Evidence
		if err := json.Unmarshal([]byte(dr.Evidence), &evidence); err != nil {
			return nil, fmt.Errorf("unmarshal evidence: %w", err)
		}
		r.Evidence = evidence
	} else {
		r.Evidence = make([]agent.Evidence, 0)
	}

	// Unmarshal Result
	if dr.Result != "" {
		r.Result = json.RawMessage(dr.Result)
	}

	// Unmarshal PendingQuestion
	if dr.PendingQuestion != "" {
		var pq agent.PendingQuestion
		if err := json.Unmarshal([]byte(dr.PendingQuestion), &pq); err != nil {
			return nil, fmt.Errorf("unmarshal pending question: %w", err)
		}
		r.PendingQuestion = &pq
	}

	return r, nil
}

// RunStore is a DynamoDB-backed implementation of run.Store.
// It provides persistent storage for agent run state and history.
type RunStore struct {
	client    Client
	tableName string
}

// RunStoreConfig holds configuration for the DynamoDB run store.
type RunStoreConfig struct {
	// TableName is the DynamoDB table name.
	TableName string

	// GSIName is the name of the Global Secondary Index for status queries.
	GSIName string
}

// NewRunStore creates a new DynamoDB run store with the given client and table name.
func NewRunStore(client Client, tableName string) *RunStore {
	return &RunStore{
		client:    client,
		tableName: tableName,
	}
}

// NewRunStoreWithConfig creates a new DynamoDB run store with full configuration.
func NewRunStoreWithConfig(client Client, cfg RunStoreConfig) *RunStore {
	return &RunStore{
		client:    client,
		tableName: cfg.TableName,
	}
}

// Save persists a new run.
// Uses PutItem with a condition to prevent overwrites.
func (s *RunStore) Save(ctx context.Context, r *agent.Run) error {
	// Validate ID
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	// Convert to DynamoDB format
	dr, err := runToDynamo(r)
	if err != nil {
		return fmt.Errorf("convert run to dynamo: %w", err)
	}

	// Marshal to DynamoDB attributes
	item, err := attributevalue.MarshalMap(dr)
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}

	// Put item with condition that ID does not exist
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	if err != nil {
		// Check for conditional check failure
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return run.ErrRunExists
		}
		return fmt.Errorf("dynamodb put failed: %w", err)
	}

	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	// Validate ID
	if id == "" {
		return nil, run.ErrInvalidRunID
	}

	// Get item
	output, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb get failed: %w", err)
	}

	// Item not found
	if output.Item == nil {
		return nil, run.ErrRunNotFound
	}

	// Unmarshal
	var dr dynamoRun
	if err := attributevalue.UnmarshalMap(output.Item, &dr); err != nil {
		return nil, fmt.Errorf("unmarshal run: %w", err)
	}

	// Convert to domain model
	r, err := dynamoToRun(&dr)
	if err != nil {
		return nil, fmt.Errorf("convert dynamo to run: %w", err)
	}

	return r, nil
}

// Update updates an existing run.
// Uses PutItem with condition to ensure the run exists.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	// Validate ID
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	// Convert to DynamoDB format
	dr, err := runToDynamo(r)
	if err != nil {
		return fmt.Errorf("convert run to dynamo: %w", err)
	}

	// Marshal to DynamoDB attributes
	item, err := attributevalue.MarshalMap(dr)
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}

	// Put item with condition that ID exists
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		// Check for conditional check failure
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return run.ErrRunNotFound
		}
		return fmt.Errorf("dynamodb put failed: %w", err)
	}

	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	// Validate ID
	if id == "" {
		return run.ErrInvalidRunID
	}

	// Delete item with condition that it exists
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		// Check for conditional check failure
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return run.ErrRunNotFound
		}
		return fmt.Errorf("dynamodb delete failed: %w", err)
	}

	return nil
}

// List returns runs matching the filter.
// Uses Scan with filter expressions and client-side sorting/pagination.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	// Build filter expression
	var filterExpr string
	exprAttrNames := make(map[string]string)
	exprAttrValues := make(map[string]types.AttributeValue)

	var conditions []string

	// Status filter
	if len(filter.Status) > 0 {
		statusValues := make([]string, len(filter.Status))
		for i, status := range filter.Status {
			placeholder := fmt.Sprintf(":s%d", i)
			statusValues[i] = placeholder
			exprAttrValues[placeholder] = &types.AttributeValueMemberS{Value: string(status)}
		}
		conditions = append(conditions, fmt.Sprintf("#status IN (%s)", strings.Join(statusValues, ", ")))
		exprAttrNames["#status"] = "status"
	}

	// States filter
	if len(filter.States) > 0 {
		stateValues := make([]string, len(filter.States))
		for i, state := range filter.States {
			placeholder := fmt.Sprintf(":st%d", i)
			stateValues[i] = placeholder
			exprAttrValues[placeholder] = &types.AttributeValueMemberS{Value: string(state)}
		}
		conditions = append(conditions, fmt.Sprintf("current_state IN (%s)", strings.Join(stateValues, ", ")))
	}

	// Time filters
	if !filter.FromTime.IsZero() {
		conditions = append(conditions, "start_time >= :from_time")
		exprAttrValues[":from_time"] = &types.AttributeValueMemberS{Value: filter.FromTime.Format(time.RFC3339Nano)}
	}
	if !filter.ToTime.IsZero() {
		conditions = append(conditions, "start_time <= :to_time")
		exprAttrValues[":to_time"] = &types.AttributeValueMemberS{Value: filter.ToTime.Format(time.RFC3339Nano)}
	}

	// Goal pattern filter
	if filter.GoalPattern != "" {
		conditions = append(conditions, "contains(goal, :goal_pattern)")
		exprAttrValues[":goal_pattern"] = &types.AttributeValueMemberS{Value: filter.GoalPattern}
	}

	// Combine conditions
	if len(conditions) > 0 {
		filterExpr = strings.Join(conditions, " AND ")
	}

	// Scan with filter
	var runs []*agent.Run
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		scanInput := &dynamodb.ScanInput{
			TableName: &s.tableName,
		}

		if filterExpr != "" {
			scanInput.FilterExpression = &filterExpr
		}
		if len(exprAttrNames) > 0 {
			scanInput.ExpressionAttributeNames = exprAttrNames
		}
		if len(exprAttrValues) > 0 {
			scanInput.ExpressionAttributeValues = exprAttrValues
		}
		if lastEvaluatedKey != nil {
			scanInput.ExclusiveStartKey = lastEvaluatedKey
		}

		output, err := s.client.Scan(ctx, scanInput)
		if err != nil {
			return nil, fmt.Errorf("dynamodb scan failed: %w", err)
		}

		// Unmarshal items
		for _, item := range output.Items {
			var dr dynamoRun
			if err := attributevalue.UnmarshalMap(item, &dr); err != nil {
				return nil, fmt.Errorf("unmarshal run: %w", err)
			}

			r, err := dynamoToRun(&dr)
			if err != nil {
				return nil, fmt.Errorf("convert dynamo to run: %w", err)
			}

			runs = append(runs, r)
		}

		// Check for more items
		if output.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = output.LastEvaluatedKey
	}

	// Client-side sorting
	s.sortRuns(runs, filter.OrderBy, filter.Descending)

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(runs) {
			return []*agent.Run{}, nil
		}
		runs = runs[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(runs) {
		runs = runs[:filter.Limit]
	}

	return runs, nil
}

// sortRuns sorts runs based on the order criteria.
func (s *RunStore) sortRuns(runs []*agent.Run, orderBy run.OrderBy, descending bool) {
	sort.Slice(runs, func(i, j int) bool {
		var less bool

		switch orderBy {
		case run.OrderByStartTime, "":
			less = runs[i].StartTime.Before(runs[j].StartTime)
		case run.OrderByEndTime:
			less = runs[i].EndTime.Before(runs[j].EndTime)
		case run.OrderByID:
			less = runs[i].ID < runs[j].ID
		case run.OrderByStatus:
			less = runs[i].Status < runs[j].Status
		default:
			less = runs[i].StartTime.Before(runs[j].StartTime)
		}

		if descending {
			return !less
		}
		return less
	})
}

// Count returns the number of runs matching the filter.
// Note: DynamoDB count operations can be expensive for large datasets.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	// Build filter expression (same as List)
	var filterExpr string
	exprAttrNames := make(map[string]string)
	exprAttrValues := make(map[string]types.AttributeValue)

	var conditions []string

	// Status filter
	if len(filter.Status) > 0 {
		statusValues := make([]string, len(filter.Status))
		for i, status := range filter.Status {
			placeholder := fmt.Sprintf(":s%d", i)
			statusValues[i] = placeholder
			exprAttrValues[placeholder] = &types.AttributeValueMemberS{Value: string(status)}
		}
		conditions = append(conditions, fmt.Sprintf("#status IN (%s)", strings.Join(statusValues, ", ")))
		exprAttrNames["#status"] = "status"
	}

	// States filter
	if len(filter.States) > 0 {
		stateValues := make([]string, len(filter.States))
		for i, state := range filter.States {
			placeholder := fmt.Sprintf(":st%d", i)
			stateValues[i] = placeholder
			exprAttrValues[placeholder] = &types.AttributeValueMemberS{Value: string(state)}
		}
		conditions = append(conditions, fmt.Sprintf("current_state IN (%s)", strings.Join(stateValues, ", ")))
	}

	// Time filters
	if !filter.FromTime.IsZero() {
		conditions = append(conditions, "start_time >= :from_time")
		exprAttrValues[":from_time"] = &types.AttributeValueMemberS{Value: filter.FromTime.Format(time.RFC3339Nano)}
	}
	if !filter.ToTime.IsZero() {
		conditions = append(conditions, "start_time <= :to_time")
		exprAttrValues[":to_time"] = &types.AttributeValueMemberS{Value: filter.ToTime.Format(time.RFC3339Nano)}
	}

	// Goal pattern filter
	if filter.GoalPattern != "" {
		conditions = append(conditions, "contains(goal, :goal_pattern)")
		exprAttrValues[":goal_pattern"] = &types.AttributeValueMemberS{Value: filter.GoalPattern}
	}

	// Combine conditions
	if len(conditions) > 0 {
		filterExpr = strings.Join(conditions, " AND ")
	}

	// Scan with count
	var totalCount int64
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		scanInput := &dynamodb.ScanInput{
			TableName: &s.tableName,
			Select:    types.SelectCount,
		}

		if filterExpr != "" {
			scanInput.FilterExpression = &filterExpr
		}
		if len(exprAttrNames) > 0 {
			scanInput.ExpressionAttributeNames = exprAttrNames
		}
		if len(exprAttrValues) > 0 {
			scanInput.ExpressionAttributeValues = exprAttrValues
		}
		if lastEvaluatedKey != nil {
			scanInput.ExclusiveStartKey = lastEvaluatedKey
		}

		output, err := s.client.Scan(ctx, scanInput)
		if err != nil {
			return 0, fmt.Errorf("dynamodb scan failed: %w", err)
		}

		totalCount += int64(output.Count)

		// Check for more items
		if output.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = output.LastEvaluatedKey
	}

	return totalCount, nil
}

// Ensure interfaces are implemented.
var (
	_ cache.Cache = (*Cache)(nil)
	_ run.Store   = (*RunStore)(nil)
)
