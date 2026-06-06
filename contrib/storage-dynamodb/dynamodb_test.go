package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/run"
)

// mockDynamoDBClient is a mock implementation of dynamoDBAPI for testing.
type mockDynamoDBClient struct {
	getItemFunc        func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	putItemFunc        func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	deleteItemFunc     func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	scanFunc           func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	batchWriteItemFunc func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

func (m *mockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if m.putItemFunc != nil {
		return m.putItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDynamoDBClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemFunc != nil {
		return m.deleteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *mockDynamoDBClient) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if m.scanFunc != nil {
		return m.scanFunc(ctx, params, optFns...)
	}
	return &dynamodb.ScanOutput{}, nil
}

func (m *mockDynamoDBClient) BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	if m.batchWriteItemFunc != nil {
		return m.batchWriteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.BatchWriteItemOutput{}, nil
}

// Cache Tests

func TestCache_Set_Get(t *testing.T) {
	store := make(map[string]map[string]types.AttributeValue)

	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			key := params.Item["pk"].(*types.AttributeValueMemberS).Value
			store[key] = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			key := params.Key["pk"].(*types.AttributeValueMemberS).Value
			item, ok := store[key]
			if !ok {
				return &dynamodb.GetItemOutput{Item: nil}, nil
			}
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	// Set a value
	err := c.Set(ctx, "test-key", []byte("test-value"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	value, found, err := c.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("Expected key to be found")
	}
	if string(value) != "test-value" {
		t.Fatalf("Expected 'test-value', got '%s'", string(value))
	}
}

func TestCache_Get_NotFound(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	value, found, err := c.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Expected key not to be found")
	}
	if value != nil {
		t.Fatal("Expected nil value")
	}
}

func TestCache_Set_WithTTL(t *testing.T) {
	var capturedItem map[string]types.AttributeValue

	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			capturedItem = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Set(ctx, "test-key", []byte("test-value"), cache.SetOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Check that TTL was set
	if _, ok := capturedItem["ttl"]; !ok {
		t.Fatal("Expected TTL attribute to be set")
	}
}

func TestCache_Delete(t *testing.T) {
	deleted := false

	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			deleted = true
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Delete(ctx, "test-key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleted {
		t.Fatal("Expected item to be deleted")
	}
}

func TestCache_Exists(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"pk": &types.AttributeValueMemberS{Value: "test-key"},
				},
			}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	exists, err := c.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected key to exist")
	}
}

func TestCache_Clear(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{
					{"pk": &types.AttributeValueMemberS{Value: "key1"}},
					{"pk": &types.AttributeValueMemberS{Value: "key2"}},
				},
			}, nil
		},
		batchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
			return &dynamodb.BatchWriteItemOutput{}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
}

func TestCache_InvalidKey(t *testing.T) {
	mock := &mockDynamoDBClient{}
	c := NewCache(mock, "test-table")
	ctx := context.Background()

	// Test Set with empty key
	err := c.Set(ctx, "", []byte("value"), cache.SetOptions{})
	if !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Expected ErrInvalidKey, got %v", err)
	}

	// Test Get with empty key
	_, _, err = c.Get(ctx, "")
	if !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Expected ErrInvalidKey, got %v", err)
	}

	// Test Delete with empty key
	err = c.Delete(ctx, "")
	if !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Expected ErrInvalidKey, got %v", err)
	}

	// Test Exists with empty key
	_, err = c.Exists(ctx, "")
	if !errors.Is(err, cache.ErrInvalidKey) {
		t.Fatalf("Expected ErrInvalidKey, got %v", err)
	}
}

// RunStore Tests

func TestRunStore_Save_Get(t *testing.T) {
	store := make(map[string]map[string]types.AttributeValue)

	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			// Check condition
			if params.ConditionExpression != nil && *params.ConditionExpression == "attribute_not_exists(id)" {
				key := params.Item["id"].(*types.AttributeValueMemberS).Value
				if _, exists := store[key]; exists {
					return nil, &types.ConditionalCheckFailedException{}
				}
			}
			key := params.Item["id"].(*types.AttributeValueMemberS).Value
			store[key] = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			key := params.Key["id"].(*types.AttributeValueMemberS).Value
			item, ok := store[key]
			if !ok {
				return &dynamodb.GetItemOutput{Item: nil}, nil
			}
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	// Create a run
	r := agent.NewRun("test-run-1", "test goal")
	r.Status = agent.RunStatusRunning
	r.AddEvidence(agent.NewSystemEvidence("test note"))
	r.SetVar("key1", "value1")

	// Save the run
	err := s.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Get the run
	retrieved, err := s.Get(ctx, "test-run-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != r.ID {
		t.Fatalf("Expected ID %s, got %s", r.ID, retrieved.ID)
	}
	if retrieved.Goal != r.Goal {
		t.Fatalf("Expected goal %s, got %s", r.Goal, retrieved.Goal)
	}
	if retrieved.Status != r.Status {
		t.Fatalf("Expected status %s, got %s", r.Status, retrieved.Status)
	}
	if len(retrieved.Evidence) != 1 {
		t.Fatalf("Expected 1 evidence, got %d", len(retrieved.Evidence))
	}
	if val, ok := retrieved.Vars["key1"]; !ok || val != "value1" {
		t.Fatalf("Expected var key1=value1, got %v", retrieved.Vars)
	}
}

func TestRunStore_Save_AlreadyExists(t *testing.T) {
	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return nil, &types.ConditionalCheckFailedException{}
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	r := agent.NewRun("test-run-1", "test goal")
	err := s.Save(ctx, r)
	if !errors.Is(err, run.ErrRunExists) {
		t.Fatalf("Expected ErrRunExists, got %v", err)
	}
}

func TestRunStore_Get_NotFound(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	if !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("Expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_Update(t *testing.T) {
	store := make(map[string]map[string]types.AttributeValue)

	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			key := params.Item["id"].(*types.AttributeValueMemberS).Value

			// Check condition for update
			if params.ConditionExpression != nil && *params.ConditionExpression == "attribute_exists(id)" {
				if _, exists := store[key]; !exists {
					return nil, &types.ConditionalCheckFailedException{}
				}
			}

			store[key] = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	// Pre-populate store
	r := agent.NewRun("test-run-1", "test goal")
	dr, _ := runToDynamo(r)
	item, _ := attributevalue.MarshalMap(dr)
	store["test-run-1"] = item

	// Update the run
	r.Status = agent.RunStatusCompleted
	r.Complete(json.RawMessage(`{"result": "success"}`))

	err := s.Update(ctx, r)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}

func TestRunStore_Update_NotFound(t *testing.T) {
	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return nil, &types.ConditionalCheckFailedException{}
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	r := agent.NewRun("test-run-1", "test goal")
	err := s.Update(ctx, r)
	if !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("Expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_Delete(t *testing.T) {
	deleted := false

	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			deleted = true
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	err := s.Delete(ctx, "test-run-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleted {
		t.Fatal("Expected run to be deleted")
	}
}

func TestRunStore_Delete_NotFound(t *testing.T) {
	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			return nil, &types.ConditionalCheckFailedException{}
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent")
	if !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("Expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_List(t *testing.T) {
	// Create test runs
	r1 := agent.NewRun("run-1", "goal 1")
	r1.Status = agent.RunStatusCompleted
	r1.StartTime = time.Now().Add(-2 * time.Hour)

	r2 := agent.NewRun("run-2", "goal 2")
	r2.Status = agent.RunStatusRunning
	r2.StartTime = time.Now().Add(-1 * time.Hour)

	r3 := agent.NewRun("run-3", "goal 3")
	r3.Status = agent.RunStatusFailed
	r3.StartTime = time.Now()

	// Convert to DynamoDB items
	dr1, _ := runToDynamo(r1)
	dr2, _ := runToDynamo(r2)
	dr3, _ := runToDynamo(r3)
	item1, _ := attributevalue.MarshalMap(dr1)
	item2, _ := attributevalue.MarshalMap(dr2)
	item3, _ := attributevalue.MarshalMap(dr3)

	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			// Simulate server-side filtering
			var items []map[string]types.AttributeValue
			allItems := []map[string]types.AttributeValue{item1, item2, item3}

			if params.FilterExpression != nil {
				// Check if filtering by status "completed"
				if params.ExpressionAttributeValues != nil {
					if statusVal, ok := params.ExpressionAttributeValues[":s0"]; ok {
						if statusStr, ok := statusVal.(*types.AttributeValueMemberS); ok && statusStr.Value == "completed" {
							items = []map[string]types.AttributeValue{item1}
						}
					}
				} else {
					items = allItems
				}
			} else {
				items = allItems
			}

			return &dynamodb.ScanOutput{
				Items: items,
			}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	// List all runs
	runs, err := s.List(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("Expected 3 runs, got %d", len(runs))
	}

	// List with status filter
	runs, err = s.List(ctx, run.ListFilter{Status: []agent.RunStatus{agent.RunStatusCompleted}})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != agent.RunStatusCompleted {
		t.Fatalf("Expected completed status, got %s", runs[0].Status)
	}

	// List with limit
	runs, err = s.List(ctx, run.ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("Expected 2 runs, got %d", len(runs))
	}
}

func TestRunStore_Count(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{
				Count: 5,
			}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	count, err := s.Count(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Fatalf("Expected count 5, got %d", count)
	}
}

func TestRunStore_InvalidRunID(t *testing.T) {
	mock := &mockDynamoDBClient{}
	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	r := &agent.Run{ID: ""}

	// Test Save with empty ID
	err := s.Save(ctx, r)
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Fatalf("Expected ErrInvalidRunID, got %v", err)
	}

	// Test Get with empty ID
	_, err = s.Get(ctx, "")
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Fatalf("Expected ErrInvalidRunID, got %v", err)
	}

	// Test Update with empty ID
	err = s.Update(ctx, r)
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Fatalf("Expected ErrInvalidRunID, got %v", err)
	}

	// Test Delete with empty ID
	err = s.Delete(ctx, "")
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Fatalf("Expected ErrInvalidRunID, got %v", err)
	}
}

// Test conversion functions
func TestRunConversion(t *testing.T) {
	// Create a complex run
	r := agent.NewRun("test-run", "complex goal")
	r.Status = agent.RunStatusCompleted
	r.CurrentState = agent.StateDone
	r.StartTime = time.Now().Add(-1 * time.Hour)
	r.EndTime = time.Now()
	r.AddEvidence(agent.NewSystemEvidence("note 1"))
	r.AddEvidence(agent.NewToolEvidence("tool1", json.RawMessage(`{"result": "ok"}`)))
	r.SetVar("var1", "value1")
	r.SetVar("var2", 42)
	r.Result = json.RawMessage(`{"final": "result"}`)
	r.AskHuman("test question?", []string{"yes", "no"})

	// Convert to DynamoDB
	dr, err := runToDynamo(r)
	if err != nil {
		t.Fatalf("runToDynamo failed: %v", err)
	}

	// Convert back
	r2, err := dynamoToRun(dr)
	if err != nil {
		t.Fatalf("dynamoToRun failed: %v", err)
	}

	// Verify fields
	if r2.ID != r.ID {
		t.Errorf("ID mismatch: expected %s, got %s", r.ID, r2.ID)
	}
	if r2.Goal != r.Goal {
		t.Errorf("Goal mismatch: expected %s, got %s", r.Goal, r2.Goal)
	}
	if r2.Status != r.Status {
		t.Errorf("Status mismatch: expected %s, got %s", r.Status, r2.Status)
	}
	if r2.CurrentState != r.CurrentState {
		t.Errorf("State mismatch: expected %s, got %s", r.CurrentState, r2.CurrentState)
	}
	if len(r2.Evidence) != len(r.Evidence) {
		t.Errorf("Evidence count mismatch: expected %d, got %d", len(r.Evidence), len(r2.Evidence))
	}
	if len(r2.Vars) != len(r.Vars) {
		t.Errorf("Vars count mismatch: expected %d, got %d", len(r.Vars), len(r2.Vars))
	}
	if string(r2.Result) != string(r.Result) {
		t.Errorf("Result mismatch: expected %s, got %s", string(r.Result), string(r2.Result))
	}
	if r2.PendingQuestion == nil {
		t.Error("Expected pending question, got nil")
	} else if r2.PendingQuestion.Question != r.PendingQuestion.Question {
		t.Errorf("Question mismatch: expected %s, got %s", r.PendingQuestion.Question, r2.PendingQuestion.Question)
	}
}

func TestCache_Get_ExpiredTTL(t *testing.T) {
	// Create an item with expired TTL
	expiredTTL := time.Now().Add(-1 * time.Hour).Unix()

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"pk":    &types.AttributeValueMemberS{Value: "test-key"},
					"value": &types.AttributeValueMemberB{Value: []byte("test-value")},
					"ttl":   &types.AttributeValueMemberN{Value: strconv.FormatInt(expiredTTL, 10)},
				},
			}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	// Get should return not found for expired items
	_, found, err := c.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Expected expired key not to be found")
	}
}

// --- Additional Cache error scenario tests ---

func TestCache_Get_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return nil, errors.New("provisioned throughput exceeded")
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	_, _, err := c.Get(ctx, "test-key")
	if err == nil {
		t.Fatal("Expected error from Get")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just verify it wraps something
	}
}

func TestCache_Set_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Set(ctx, "test-key", []byte("value"), cache.SetOptions{})
	if err == nil {
		t.Fatal("Expected error from Set")
	}
}

func TestCache_Delete_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			return nil, errors.New("internal server error")
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Delete(ctx, "test-key")
	if err == nil {
		t.Fatal("Expected error from Delete")
	}
}

func TestCache_Exists_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	_, err := c.Exists(ctx, "test-key")
	if err == nil {
		t.Fatal("Expected error from Exists")
	}
}

func TestCache_Exists_NotFound(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	exists, err := c.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("Expected key not to exist")
	}
}

func TestCache_Exists_ExpiredTTL(t *testing.T) {
	expiredTTL := time.Now().Add(-1 * time.Hour).Unix()

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"pk":  &types.AttributeValueMemberS{Value: "test-key"},
					"ttl": &types.AttributeValueMemberN{Value: strconv.FormatInt(expiredTTL, 10)},
				},
			}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	exists, err := c.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("Expected expired key not to exist")
	}
}

func TestCache_Clear_DynamoDBScanError(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return nil, errors.New("scan error")
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Clear(ctx)
	if err == nil {
		t.Fatal("Expected error from Clear")
	}
}

func TestCache_Clear_BatchWriteError(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{
					{"pk": &types.AttributeValueMemberS{Value: "key1"}},
				},
			}, nil
		},
		batchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
			return nil, errors.New("batch write error")
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Clear(ctx)
	if err == nil {
		t.Fatal("Expected error from Clear batch write")
	}
}

func TestCache_Clear_EmptyTable(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{},
			}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear on empty table failed: %v", err)
	}
}

func TestCache_Clear_Pagination(t *testing.T) {
	callCount := 0
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			callCount++
			if callCount == 1 {
				return &dynamodb.ScanOutput{
					Items: []map[string]types.AttributeValue{
						{"pk": &types.AttributeValueMemberS{Value: "key1"}},
					},
					LastEvaluatedKey: map[string]types.AttributeValue{
						"pk": &types.AttributeValueMemberS{Value: "key1"},
					},
				}, nil
			}
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{
					{"pk": &types.AttributeValueMemberS{Value: "key2"}},
				},
			}, nil
		},
		batchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
			return &dynamodb.BatchWriteItemOutput{}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear with pagination failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 scan calls for pagination, got %d", callCount)
	}
}

func TestCache_Get_CancelledContext(t *testing.T) {
	mock := &mockDynamoDBClient{}
	c := NewCache(mock, "test-table")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.Get(ctx, "test-key")
	if err == nil {
		t.Fatal("Expected error from cancelled context")
	}
}

func TestCache_Get_MissingValueAttribute(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"pk": &types.AttributeValueMemberS{Value: "test-key"},
					// No "value" attribute
				},
			}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	_, _, err := c.Get(ctx, "test-key")
	if err == nil {
		t.Fatal("Expected error for missing value attribute")
	}
}

func TestCache_Get_WrongValueType(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"pk":    &types.AttributeValueMemberS{Value: "test-key"},
					"value": &types.AttributeValueMemberS{Value: "not-binary"},
				},
			}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	_, _, err := c.Get(ctx, "test-key")
	if err == nil {
		t.Fatal("Expected error for wrong value type")
	}
}

func TestCache_Set_WithoutTTL(t *testing.T) {
	var capturedItem map[string]types.AttributeValue

	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			capturedItem = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	c := NewCache(mock, "test-table")
	ctx := context.Background()

	err := c.Set(ctx, "test-key", []byte("test-value"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify no TTL attribute when TTL is 0
	if _, ok := capturedItem["ttl"]; ok {
		t.Fatal("Expected no TTL attribute when TTL is 0")
	}
}

// --- Additional RunStore error scenario tests ---

func TestRunStore_Save_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	r := agent.NewRun("test-run-1", "test goal")
	err := s.Save(ctx, r)
	if err == nil {
		t.Fatal("Expected error from Save")
	}
}

func TestRunStore_Get_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	_, err := s.Get(ctx, "test-run-1")
	if err == nil {
		t.Fatal("Expected error from Get")
	}
}

func TestRunStore_Update_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	r := agent.NewRun("test-run-1", "test goal")
	err := s.Update(ctx, r)
	if err == nil {
		t.Fatal("Expected error from Update")
	}
}

func TestRunStore_Delete_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	err := s.Delete(ctx, "test-run-1")
	if err == nil {
		t.Fatal("Expected error from Delete")
	}
}

func TestRunStore_List_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return nil, errors.New("scan error")
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	_, err := s.List(ctx, run.ListFilter{})
	if err == nil {
		t.Fatal("Expected error from List")
	}
}

func TestRunStore_Count_DynamoDBError(t *testing.T) {
	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return nil, errors.New("scan error")
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	_, err := s.Count(ctx, run.ListFilter{})
	if err == nil {
		t.Fatal("Expected error from Count")
	}
}

func TestRunStore_List_WithOffset(t *testing.T) {
	r1 := agent.NewRun("run-1", "goal 1")
	r1.StartTime = time.Now().Add(-2 * time.Hour)

	r2 := agent.NewRun("run-2", "goal 2")
	r2.StartTime = time.Now().Add(-1 * time.Hour)

	r3 := agent.NewRun("run-3", "goal 3")
	r3.StartTime = time.Now()

	dr1, _ := runToDynamo(r1)
	dr2, _ := runToDynamo(r2)
	dr3, _ := runToDynamo(r3)
	item1, _ := attributevalue.MarshalMap(dr1)
	item2, _ := attributevalue.MarshalMap(dr2)
	item3, _ := attributevalue.MarshalMap(dr3)

	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{item1, item2, item3},
			}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	// List with offset=1
	runs, err := s.List(ctx, run.ListFilter{Offset: 1})
	if err != nil {
		t.Fatalf("List with offset failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("Expected 2 runs with offset, got %d", len(runs))
	}

	// List with offset exceeding length
	runs, err = s.List(ctx, run.ListFilter{Offset: 10})
	if err != nil {
		t.Fatalf("List with large offset failed: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("Expected 0 runs with large offset, got %d", len(runs))
	}
}

func TestRunStore_List_Descending(t *testing.T) {
	r1 := agent.NewRun("run-1", "goal 1")
	r1.StartTime = time.Now().Add(-2 * time.Hour)

	r2 := agent.NewRun("run-2", "goal 2")
	r2.StartTime = time.Now().Add(-1 * time.Hour)

	dr1, _ := runToDynamo(r1)
	dr2, _ := runToDynamo(r2)
	item1, _ := attributevalue.MarshalMap(dr1)
	item2, _ := attributevalue.MarshalMap(dr2)

	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{item1, item2},
			}, nil
		},
	}

	s := NewRunStore(mock, "test-table")
	ctx := context.Background()

	runs, err := s.List(ctx, run.ListFilter{Descending: true})
	if err != nil {
		t.Fatalf("List descending failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("Expected 2 runs, got %d", len(runs))
	}
	// With descending order by start time, run-2 (newer) should come first
	if runs[0].ID != "run-2" {
		t.Errorf("Expected run-2 first in descending order, got %s", runs[0].ID)
	}
}

// --- Interface compliance tests ---

func TestCache_ImplementsCacheInterface(t *testing.T) {
	var _ cache.Cache = (*Cache)(nil)
}

func TestRunStore_ImplementsRunStoreInterface(t *testing.T) {
	var _ run.Store = (*RunStore)(nil)
}

// --- Constructor tests ---

func TestNewCacheWithConfig(t *testing.T) {
	mock := &mockDynamoDBClient{}
	c := NewCacheWithConfig(mock, CacheConfig{
		TableName:        "custom-table",
		TTLAttributeName: "custom-ttl",
		KeyPrefix:        "prefix:",
	})
	if c == nil {
		t.Fatal("Expected non-nil cache")
	}
}

func TestNewRunStoreWithConfig(t *testing.T) {
	mock := &mockDynamoDBClient{}
	s := NewRunStoreWithConfig(mock, RunStoreConfig{
		TableName: "custom-runs-table",
		GSIName:   "status-gsi",
	})
	if s == nil {
		t.Fatal("Expected non-nil run store")
	}
}

// --- Run conversion edge cases ---

func TestRunConversion_EmptyRun(t *testing.T) {
	r := agent.NewRun("minimal-run", "minimal goal")

	dr, err := runToDynamo(r)
	if err != nil {
		t.Fatalf("runToDynamo failed: %v", err)
	}

	r2, err := dynamoToRun(dr)
	if err != nil {
		t.Fatalf("dynamoToRun failed: %v", err)
	}

	if r2.ID != r.ID {
		t.Errorf("ID mismatch: expected %s, got %s", r.ID, r2.ID)
	}
	if r2.Goal != r.Goal {
		t.Errorf("Goal mismatch: expected %s, got %s", r.Goal, r2.Goal)
	}
	if len(r2.Evidence) != 0 {
		t.Errorf("Expected empty evidence, got %d", len(r2.Evidence))
	}
	if r2.Vars == nil {
		t.Error("Expected non-nil vars map")
	}
}

func TestRunConversion_NilVars(t *testing.T) {
	r := agent.NewRun("test-run", "test goal")
	r.Vars = nil

	dr, err := runToDynamo(r)
	if err != nil {
		t.Fatalf("runToDynamo failed: %v", err)
	}

	if dr.Vars != "" {
		t.Errorf("Expected empty vars string for nil vars, got %q", dr.Vars)
	}

	r2, err := dynamoToRun(dr)
	if err != nil {
		t.Fatalf("dynamoToRun failed: %v", err)
	}

	if r2.Vars == nil {
		t.Error("Expected non-nil vars map after round-trip")
	}
}

func TestDynamoToRun_InvalidStartTime(t *testing.T) {
	dr := &dynamoRun{
		ID:        "test-run",
		Goal:      "test",
		StartTime: "not-a-time",
		Evidence:  "[]",
	}

	_, err := dynamoToRun(dr)
	if err == nil {
		t.Fatal("Expected error for invalid start time")
	}
}

func TestDynamoToRun_InvalidEndTime(t *testing.T) {
	dr := &dynamoRun{
		ID:        "test-run",
		Goal:      "test",
		StartTime: time.Now().Format(time.RFC3339Nano),
		EndTime:   "not-a-time",
		Evidence:  "[]",
	}

	_, err := dynamoToRun(dr)
	if err == nil {
		t.Fatal("Expected error for invalid end time")
	}
}

func TestDynamoToRun_InvalidEvidence(t *testing.T) {
	dr := &dynamoRun{
		ID:        "test-run",
		Goal:      "test",
		StartTime: time.Now().Format(time.RFC3339Nano),
		Evidence:  "not-valid-json",
	}

	_, err := dynamoToRun(dr)
	if err == nil {
		t.Fatal("Expected error for invalid evidence JSON")
	}
}

func TestDynamoToRun_InvalidVars(t *testing.T) {
	dr := &dynamoRun{
		ID:        "test-run",
		Goal:      "test",
		StartTime: time.Now().Format(time.RFC3339Nano),
		Vars:      "not-valid-json",
		Evidence:  "[]",
	}

	_, err := dynamoToRun(dr)
	if err == nil {
		t.Fatal("Expected error for invalid vars JSON")
	}
}

func TestDynamoToRun_InvalidPendingQuestion(t *testing.T) {
	dr := &dynamoRun{
		ID:              "test-run",
		Goal:            "test",
		StartTime:       time.Now().Format(time.RFC3339Nano),
		Evidence:        "[]",
		PendingQuestion: "not-valid-json",
	}

	_, err := dynamoToRun(dr)
	if err == nil {
		t.Fatal("Expected error for invalid pending question JSON")
	}
}
