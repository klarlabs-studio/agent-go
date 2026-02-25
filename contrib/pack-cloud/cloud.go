// Package cloud provides cloud storage tools for agent-go.
//
// This pack includes tools for cloud storage operations:
//   - cloud_upload: Upload a file to cloud storage
//   - cloud_download: Download a file from cloud storage
//   - cloud_list: List objects in a bucket/container
//   - cloud_delete: Delete an object from cloud storage
//   - cloud_copy: Copy an object within or between buckets
//   - cloud_presign: Generate a presigned URL for an object
//   - cloud_metadata: Get object metadata
//
// Provider-agnostic via the ObjectStore interface. Implement it for
// AWS S3, Google Cloud Storage, Azure Blob Storage, or any compatible backend.
package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ObjectMeta holds metadata for a stored object.
type ObjectMeta struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type,omitempty"`
	ETag         string            `json:"etag,omitempty"`
	LastModified time.Time         `json:"last_modified"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ListResult contains the results of a list operation.
type ListResult struct {
	Objects    []ObjectMeta `json:"objects"`
	Truncated  bool         `json:"truncated"`
	NextMarker string       `json:"next_marker,omitempty"`
}

// ObjectStore is the provider-agnostic interface for cloud storage operations.
type ObjectStore interface {
	// Upload stores data at the given key in the bucket.
	Upload(ctx context.Context, bucket, key string, data io.Reader, contentType string, metadata map[string]string) error

	// Download retrieves the data stored at the given key.
	Download(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectMeta, error)

	// List returns objects in the bucket matching the optional prefix.
	List(ctx context.Context, bucket, prefix, marker string, maxKeys int) (*ListResult, error)

	// Delete removes the object at the given key.
	Delete(ctx context.Context, bucket, key string) error

	// Copy copies an object from src to dst.
	Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error

	// Presign generates a presigned URL valid for the given duration.
	Presign(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)

	// Metadata returns the metadata for the given object without downloading it.
	Metadata(ctx context.Context, bucket, key string) (*ObjectMeta, error)
}

// Config holds cloud storage pack configuration.
type Config struct {
	// Store is the ObjectStore implementation. Required.
	Store ObjectStore

	// DefaultBucket is used when no bucket is specified in tool input.
	DefaultBucket string

	// MaxListKeys is the maximum number of objects returned by list (default: 1000).
	MaxListKeys int

	// MaxDownloadSize is the maximum download size in bytes (default: 50MB).
	MaxDownloadSize int64
}

type cloudPack struct {
	cfg Config
}

// Pack returns the cloud storage tools pack.
func Pack(cfg Config) *pack.Pack {
	if cfg.MaxListKeys == 0 {
		cfg.MaxListKeys = 1000
	}
	if cfg.MaxDownloadSize == 0 {
		cfg.MaxDownloadSize = 50 * 1024 * 1024 // 50MB
	}

	p := &cloudPack{cfg: cfg}

	return pack.NewBuilder("cloud").
		WithDescription("Cloud storage tools for S3, GCS, and Azure Blob Storage").
		WithVersion("0.1.0").
		AddTools(
			p.cloudUpload(),
			p.cloudDownload(),
			p.cloudList(),
			p.cloudDelete(),
			p.cloudCopy(),
			p.cloudPresign(),
			p.cloudMetadata(),
		).
		AllowInState(agent.StateExplore, "cloud_list", "cloud_metadata", "cloud_download").
		AllowInState(agent.StateAct, "cloud_upload", "cloud_download", "cloud_list", "cloud_delete", "cloud_copy", "cloud_presign", "cloud_metadata").
		AllowInState(agent.StateValidate, "cloud_list", "cloud_metadata").
		Build()
}

func (p *cloudPack) bucket(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if p.cfg.DefaultBucket != "" {
		return p.cfg.DefaultBucket, nil
	}
	return "", fmt.Errorf("bucket is required (set in config or params)")
}

func (p *cloudPack) cloudUpload() tool.Tool {
	return tool.NewBuilder("cloud_upload").
		WithDescription("Upload a file to cloud storage").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bucket      string            `json:"bucket,omitempty"`
				Key         string            `json:"key"`
				Data        string            `json:"data"`
				ContentType string            `json:"content_type,omitempty"`
				Metadata    map[string]string `json:"metadata,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Key == "" {
				return tool.Result{}, fmt.Errorf("key is required")
			}
			if params.Data == "" {
				return tool.Result{}, fmt.Errorf("data is required")
			}

			bucket, err := p.bucket(params.Bucket)
			if err != nil {
				return tool.Result{}, err
			}

			contentType := params.ContentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			reader := strings.NewReader(params.Data)
			if err := p.cfg.Store.Upload(ctx, bucket, params.Key, reader, contentType, params.Metadata); err != nil {
				return tool.Result{}, fmt.Errorf("upload failed: %w", err)
			}

			result := map[string]any{
				"success":      true,
				"bucket":       bucket,
				"key":          params.Key,
				"size":         len(params.Data),
				"content_type": contentType,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *cloudPack) cloudDownload() tool.Tool {
	return tool.NewBuilder("cloud_download").
		WithDescription("Download a file from cloud storage").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bucket string `json:"bucket,omitempty"`
				Key    string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Key == "" {
				return tool.Result{}, fmt.Errorf("key is required")
			}

			bucket, err := p.bucket(params.Bucket)
			if err != nil {
				return tool.Result{}, err
			}

			rc, meta, err := p.cfg.Store.Download(ctx, bucket, params.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("download failed: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(io.LimitReader(rc, p.cfg.MaxDownloadSize))
			if err != nil {
				return tool.Result{}, fmt.Errorf("read failed: %w", err)
			}

			result := map[string]any{
				"bucket": bucket,
				"key":    params.Key,
				"data":   string(data),
				"size":   len(data),
			}
			if meta != nil {
				result["content_type"] = meta.ContentType
				result["etag"] = meta.ETag
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *cloudPack) cloudList() tool.Tool {
	return tool.NewBuilder("cloud_list").
		WithDescription("List objects in a cloud storage bucket or container").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bucket  string `json:"bucket,omitempty"`
				Prefix  string `json:"prefix,omitempty"`
				Marker  string `json:"marker,omitempty"`
				MaxKeys int    `json:"max_keys,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			bucket, err := p.bucket(params.Bucket)
			if err != nil {
				return tool.Result{}, err
			}

			maxKeys := params.MaxKeys
			if maxKeys <= 0 || maxKeys > p.cfg.MaxListKeys {
				maxKeys = p.cfg.MaxListKeys
			}

			lr, err := p.cfg.Store.List(ctx, bucket, params.Prefix, params.Marker, maxKeys)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list failed: %w", err)
			}

			result := map[string]any{
				"bucket":    bucket,
				"objects":   lr.Objects,
				"count":     len(lr.Objects),
				"truncated": lr.Truncated,
			}
			if lr.NextMarker != "" {
				result["next_marker"] = lr.NextMarker
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *cloudPack) cloudDelete() tool.Tool {
	return tool.NewBuilder("cloud_delete").
		WithDescription("Delete an object from cloud storage").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bucket string `json:"bucket,omitempty"`
				Key    string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Key == "" {
				return tool.Result{}, fmt.Errorf("key is required")
			}

			bucket, err := p.bucket(params.Bucket)
			if err != nil {
				return tool.Result{}, err
			}

			if err := p.cfg.Store.Delete(ctx, bucket, params.Key); err != nil {
				return tool.Result{}, fmt.Errorf("delete failed: %w", err)
			}

			result := map[string]any{
				"success": true,
				"bucket":  bucket,
				"key":     params.Key,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *cloudPack) cloudCopy() tool.Tool {
	return tool.NewBuilder("cloud_copy").
		WithDescription("Copy an object within or between buckets").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				SrcBucket string `json:"src_bucket,omitempty"`
				SrcKey    string `json:"src_key"`
				DstBucket string `json:"dst_bucket,omitempty"`
				DstKey    string `json:"dst_key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.SrcKey == "" {
				return tool.Result{}, fmt.Errorf("src_key is required")
			}
			if params.DstKey == "" {
				return tool.Result{}, fmt.Errorf("dst_key is required")
			}

			srcBucket, err := p.bucket(params.SrcBucket)
			if err != nil {
				return tool.Result{}, fmt.Errorf("src_bucket: %w", err)
			}
			dstBucket, err := p.bucket(params.DstBucket)
			if err != nil {
				return tool.Result{}, fmt.Errorf("dst_bucket: %w", err)
			}

			if err := p.cfg.Store.Copy(ctx, srcBucket, params.SrcKey, dstBucket, params.DstKey); err != nil {
				return tool.Result{}, fmt.Errorf("copy failed: %w", err)
			}

			result := map[string]any{
				"success":    true,
				"src_bucket": srcBucket,
				"src_key":    params.SrcKey,
				"dst_bucket": dstBucket,
				"dst_key":    params.DstKey,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *cloudPack) cloudPresign() tool.Tool {
	return tool.NewBuilder("cloud_presign").
		WithDescription("Generate a presigned URL for temporary object access").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bucket    string `json:"bucket,omitempty"`
				Key       string `json:"key"`
				ExpirySec int    `json:"expiry_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Key == "" {
				return tool.Result{}, fmt.Errorf("key is required")
			}

			bucket, err := p.bucket(params.Bucket)
			if err != nil {
				return tool.Result{}, err
			}

			expiry := time.Duration(params.ExpirySec) * time.Second
			if expiry <= 0 {
				expiry = 15 * time.Minute
			}

			url, err := p.cfg.Store.Presign(ctx, bucket, params.Key, expiry)
			if err != nil {
				return tool.Result{}, fmt.Errorf("presign failed: %w", err)
			}

			result := map[string]any{
				"url":            url,
				"bucket":         bucket,
				"key":            params.Key,
				"expiry_seconds": int(expiry.Seconds()),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *cloudPack) cloudMetadata() tool.Tool {
	return tool.NewBuilder("cloud_metadata").
		WithDescription("Get metadata for a cloud storage object").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bucket string `json:"bucket,omitempty"`
				Key    string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Key == "" {
				return tool.Result{}, fmt.Errorf("key is required")
			}

			bucket, err := p.bucket(params.Bucket)
			if err != nil {
				return tool.Result{}, err
			}

			meta, err := p.cfg.Store.Metadata(ctx, bucket, params.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("metadata failed: %w", err)
			}

			result := map[string]any{
				"bucket":        bucket,
				"key":           meta.Key,
				"size":          meta.Size,
				"content_type":  meta.ContentType,
				"etag":          meta.ETag,
				"last_modified": meta.LastModified.Format(time.RFC3339),
				"metadata":      meta.Metadata,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
