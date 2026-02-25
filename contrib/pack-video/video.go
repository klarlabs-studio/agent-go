// Package video provides video processing tools for agent-go.
//
// The pack uses an interface-based approach, allowing any video processing
// engine (FFmpeg, cloud services, etc.) to be plugged in.
package video

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// VideoEngine provides video processing capabilities.
type VideoEngine interface {
	ExtractFrames(ctx context.Context, input VideoInput, opts FrameOptions) ([]Frame, error)
	Transcode(ctx context.Context, input VideoInput, opts TranscodeOptions) (*VideoOutput, error)
	GenerateThumbnail(ctx context.Context, input VideoInput, timestamp float64) (*Frame, error)
	Clip(ctx context.Context, input VideoInput, start, end float64) (*VideoOutput, error)
	Merge(ctx context.Context, inputs []VideoInput) (*VideoOutput, error)
	AddSubtitles(ctx context.Context, input VideoInput, subtitles string, format string) (*VideoOutput, error)
	GetInfo(ctx context.Context, input VideoInput) (*VideoInfo, error)
}

// VideoInput represents video data for processing.
type VideoInput struct {
	Data   string `json:"data,omitempty"`
	URL    string `json:"url,omitempty"`
	Path   string `json:"path,omitempty"`
	Format string `json:"format,omitempty"`
}

// VideoOutput represents produced video data.
type VideoOutput struct {
	Data     string  `json:"data,omitempty"`
	Format   string  `json:"format"`
	Duration float64 `json:"duration_seconds"`
	Size     int64   `json:"size_bytes"`
}

// Frame represents a video frame.
type Frame struct {
	Data      string  `json:"data"` // base64 image
	Timestamp float64 `json:"timestamp"`
	Format    string  `json:"format"`
}

// FrameOptions configures frame extraction.
type FrameOptions struct {
	Interval float64 `json:"interval,omitempty"`
	MaxFrames int    `json:"max_frames,omitempty"`
	Format    string `json:"format,omitempty"`
}

// TranscodeOptions configures video transcoding.
type TranscodeOptions struct {
	Format     string `json:"format"`
	Resolution string `json:"resolution,omitempty"`
	Bitrate    string `json:"bitrate,omitempty"`
	Codec      string `json:"codec,omitempty"`
	FPS        int    `json:"fps,omitempty"`
}

// VideoInfo contains video file metadata.
type VideoInfo struct {
	Format     string  `json:"format"`
	Duration   float64 `json:"duration_seconds"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FPS        float64 `json:"fps"`
	Bitrate    int     `json:"bitrate"`
	Codec      string  `json:"codec,omitempty"`
	AudioCodec string  `json:"audio_codec,omitempty"`
	Size       int64   `json:"size_bytes"`
}

// Config holds video pack configuration.
type Config struct {
	Engine VideoEngine
}

// Pack returns the video processing tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &videoPack{cfg: cfg}

	return pack.NewBuilder("video").
		WithDescription("Video processing tools: extract frames, transcode, thumbnail, clip, merge, subtitles").
		WithVersion("1.0.0").
		AddTools(
			p.extractFramesTool(), p.transcodeTool(), p.thumbnailTool(),
			p.clipTool(), p.mergeTool(), p.subtitlesTool(), p.infoTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type videoPack struct{ cfg Config }

func (p *videoPack) extractFramesTool() tool.Tool {
	return tool.NewBuilder("video_extract_frames").
		WithDescription("Extract frames from a video at regular intervals").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				VideoInput
				Interval  float64 `json:"interval,omitempty"`
				MaxFrames int     `json:"max_frames,omitempty"`
				Format    string  `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}
			frames, err := p.cfg.Engine.ExtractFrames(ctx, in.VideoInput, FrameOptions{
				Interval: in.Interval, MaxFrames: in.MaxFrames, Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("extract frames failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"count": len(frames), "frames": frames})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *videoPack) transcodeTool() tool.Tool {
	return tool.NewBuilder("video_transcode").
		WithDescription("Transcode a video to a different format or resolution").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				VideoInput
				TargetFormat string `json:"target_format"`
				Resolution   string `json:"resolution,omitempty"`
				Bitrate      string `json:"bitrate,omitempty"`
				Codec        string `json:"codec,omitempty"`
				FPS          int    `json:"fps,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}
			if in.TargetFormat == "" {
				return tool.Result{}, fmt.Errorf("target_format is required")
			}
			result, err := p.cfg.Engine.Transcode(ctx, in.VideoInput, TranscodeOptions{
				Format: in.TargetFormat, Resolution: in.Resolution,
				Bitrate: in.Bitrate, Codec: in.Codec, FPS: in.FPS,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("transcode failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *videoPack) thumbnailTool() tool.Tool {
	return tool.NewBuilder("video_generate_thumbnail").
		WithDescription("Generate a thumbnail from a video at a specific timestamp").
		ReadOnly().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				VideoInput
				Timestamp float64 `json:"timestamp,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}
			frame, err := p.cfg.Engine.GenerateThumbnail(ctx, in.VideoInput, in.Timestamp)
			if err != nil {
				return tool.Result{}, fmt.Errorf("thumbnail failed: %w", err)
			}
			output, _ := json.Marshal(frame)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *videoPack) clipTool() tool.Tool {
	return tool.NewBuilder("video_clip").
		WithDescription("Extract a clip from a video between timestamps").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				VideoInput
				Start float64 `json:"start"`
				End   float64 `json:"end"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}
			if in.End <= in.Start {
				return tool.Result{}, fmt.Errorf("end must be greater than start")
			}
			result, err := p.cfg.Engine.Clip(ctx, in.VideoInput, in.Start, in.End)
			if err != nil {
				return tool.Result{}, fmt.Errorf("clip failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *videoPack) mergeTool() tool.Tool {
	return tool.NewBuilder("video_merge").
		WithDescription("Merge multiple videos into one").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Videos []VideoInput `json:"videos"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.Videos) < 2 {
				return tool.Result{}, fmt.Errorf("at least 2 videos are required")
			}
			result, err := p.cfg.Engine.Merge(ctx, in.Videos)
			if err != nil {
				return tool.Result{}, fmt.Errorf("merge failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *videoPack) subtitlesTool() tool.Tool {
	return tool.NewBuilder("video_add_subtitles").
		WithDescription("Add subtitles to a video").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				VideoInput
				Subtitles string `json:"subtitles"`
				Format    string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}
			if in.Subtitles == "" {
				return tool.Result{}, fmt.Errorf("subtitles is required")
			}
			if in.Format == "" {
				in.Format = "srt"
			}
			result, err := p.cfg.Engine.AddSubtitles(ctx, in.VideoInput, in.Subtitles, in.Format)
			if err != nil {
				return tool.Result{}, fmt.Errorf("add subtitles failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *videoPack) infoTool() tool.Tool {
	return tool.NewBuilder("video_get_info").
		WithDescription("Get metadata about a video file").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in VideoInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}
			info, err := p.cfg.Engine.GetInfo(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get info failed: %w", err)
			}
			output, _ := json.Marshal(info)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
