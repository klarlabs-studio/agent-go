// Package audio provides audio processing tools for agent-go.
//
// The pack uses an interface-based approach, allowing any audio processing
// engine (FFmpeg, cloud speech APIs, etc.) to be plugged in.
package audio

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// AudioEngine provides audio processing capabilities.
type AudioEngine interface {
	// Transcribe converts speech audio to text.
	Transcribe(ctx context.Context, input AudioInput, opts TranscribeOptions) (*TranscribeResult, error)

	// Synthesize converts text to speech audio.
	Synthesize(ctx context.Context, text string, opts SynthesizeOptions) (*AudioOutput, error)

	// ConvertFormat converts audio between formats.
	ConvertFormat(ctx context.Context, input AudioInput, targetFormat string, opts ConvertOptions) (*AudioOutput, error)

	// ExtractAudio extracts the audio track from a video.
	ExtractAudio(ctx context.Context, videoInput string, format string) (*AudioOutput, error)

	// DetectLanguage detects the spoken language in audio.
	DetectLanguage(ctx context.Context, input AudioInput) (*LanguageResult, error)

	// GetInfo returns metadata about an audio file.
	GetInfo(ctx context.Context, input AudioInput) (*AudioInfo, error)
}

// AudioInput represents audio data for processing.
type AudioInput struct {
	Data   string `json:"data,omitempty"`   // base64-encoded audio
	URL    string `json:"url,omitempty"`    // URL to audio file
	Path   string `json:"path,omitempty"`   // local file path
	Format string `json:"format,omitempty"` // audio format hint
}

// AudioOutput represents produced audio data.
type AudioOutput struct {
	Data     string  `json:"data,omitempty"` // base64-encoded audio
	Format   string  `json:"format"`
	Duration float64 `json:"duration_seconds"`
	Size     int64   `json:"size_bytes"`
}

// TranscribeOptions configures speech-to-text.
type TranscribeOptions struct {
	Language      string `json:"language,omitempty"`
	Model         string `json:"model,omitempty"`
	Timestamps    bool   `json:"timestamps,omitempty"`
	SpeakerLabels bool   `json:"speaker_labels,omitempty"`
}

// TranscribeResult contains transcription output.
type TranscribeResult struct {
	Text       string    `json:"text"`
	Language   string    `json:"language,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
	Duration   float64   `json:"duration_seconds,omitempty"`
	Segments   []Segment `json:"segments,omitempty"`
}

// Segment represents a timed segment of transcription.
type Segment struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Speaker    string  `json:"speaker,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// SynthesizeOptions configures text-to-speech.
type SynthesizeOptions struct {
	Voice    string  `json:"voice,omitempty"`
	Language string  `json:"language,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
	Format   string  `json:"format,omitempty"`
}

// ConvertOptions configures audio format conversion.
type ConvertOptions struct {
	SampleRate int    `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	Bitrate    string `json:"bitrate,omitempty"`
}

// LanguageResult contains language detection output.
type LanguageResult struct {
	Language   string             `json:"language"`
	Confidence float64            `json:"confidence"`
	Scores     map[string]float64 `json:"scores,omitempty"`
}

// AudioInfo contains audio file metadata.
type AudioInfo struct {
	Format     string  `json:"format"`
	Duration   float64 `json:"duration_seconds"`
	SampleRate int     `json:"sample_rate"`
	Channels   int     `json:"channels"`
	Bitrate    int     `json:"bitrate"`
	Size       int64   `json:"size_bytes"`
	Codec      string  `json:"codec,omitempty"`
}

// Config holds audio pack configuration.
type Config struct {
	// Engine is the audio processing engine (required).
	Engine AudioEngine
}

// Pack returns the audio processing tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &audioPack{cfg: cfg}

	return pack.NewBuilder("audio").
		WithDescription("Audio processing tools: transcription, synthesis, format conversion, language detection").
		WithVersion("1.0.0").
		AddTools(
			p.transcribeTool(),
			p.synthesizeTool(),
			p.convertFormatTool(),
			p.extractAudioTool(),
			p.detectLanguageTool(),
			p.getInfoTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type audioPack struct {
	cfg Config
}

func (p *audioPack) transcribeTool() tool.Tool {
	return tool.NewBuilder("audio_transcribe").
		WithDescription("Convert speech audio to text").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data          string `json:"data,omitempty"`
				URL           string `json:"url,omitempty"`
				Path          string `json:"path,omitempty"`
				Format        string `json:"format,omitempty"`
				Language      string `json:"language,omitempty"`
				Model         string `json:"model,omitempty"`
				Timestamps    bool   `json:"timestamps,omitempty"`
				SpeakerLabels bool   `json:"speaker_labels,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}

			result, err := p.cfg.Engine.Transcribe(ctx, AudioInput{
				Data: in.Data, URL: in.URL, Path: in.Path, Format: in.Format,
			}, TranscribeOptions{
				Language: in.Language, Model: in.Model,
				Timestamps: in.Timestamps, SpeakerLabels: in.SpeakerLabels,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("transcription failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *audioPack) synthesizeTool() tool.Tool {
	return tool.NewBuilder("audio_synthesize").
		WithDescription("Convert text to speech audio").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Text     string  `json:"text"`
				Voice    string  `json:"voice,omitempty"`
				Language string  `json:"language,omitempty"`
				Speed    float64 `json:"speed,omitempty"`
				Format   string  `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Text == "" {
				return tool.Result{}, fmt.Errorf("text is required")
			}

			result, err := p.cfg.Engine.Synthesize(ctx, in.Text, SynthesizeOptions{
				Voice: in.Voice, Language: in.Language,
				Speed: in.Speed, Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("synthesis failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *audioPack) convertFormatTool() tool.Tool {
	return tool.NewBuilder("audio_convert_format").
		WithDescription("Convert audio between formats").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data         string `json:"data,omitempty"`
				URL          string `json:"url,omitempty"`
				Path         string `json:"path,omitempty"`
				Format       string `json:"format,omitempty"`
				TargetFormat string `json:"target_format"`
				SampleRate   int    `json:"sample_rate,omitempty"`
				Channels     int    `json:"channels,omitempty"`
				Bitrate      string `json:"bitrate,omitempty"`
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

			result, err := p.cfg.Engine.ConvertFormat(ctx, AudioInput{
				Data: in.Data, URL: in.URL, Path: in.Path, Format: in.Format,
			}, in.TargetFormat, ConvertOptions{
				SampleRate: in.SampleRate, Channels: in.Channels, Bitrate: in.Bitrate,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("conversion failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *audioPack) extractAudioTool() tool.Tool {
	return tool.NewBuilder("audio_extract_audio").
		WithDescription("Extract audio track from a video file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Video  string `json:"video"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Video == "" {
				return tool.Result{}, fmt.Errorf("video is required (URL or base64)")
			}
			if in.Format == "" {
				in.Format = "mp3"
			}

			result, err := p.cfg.Engine.ExtractAudio(ctx, in.Video, in.Format)
			if err != nil {
				return tool.Result{}, fmt.Errorf("audio extraction failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *audioPack) detectLanguageTool() tool.Tool {
	return tool.NewBuilder("audio_detect_language").
		WithDescription("Detect the spoken language in audio").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data   string `json:"data,omitempty"`
				URL    string `json:"url,omitempty"`
				Path   string `json:"path,omitempty"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}

			result, err := p.cfg.Engine.DetectLanguage(ctx, AudioInput{
				Data: in.Data, URL: in.URL, Path: in.Path, Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("language detection failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *audioPack) getInfoTool() tool.Tool {
	return tool.NewBuilder("audio_get_info").
		WithDescription("Get metadata about an audio file").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Data   string `json:"data,omitempty"`
				URL    string `json:"url,omitempty"`
				Path   string `json:"path,omitempty"`
				Format string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Data == "" && in.URL == "" && in.Path == "" {
				return tool.Result{}, fmt.Errorf("data, url, or path is required")
			}

			info, err := p.cfg.Engine.GetInfo(ctx, AudioInput{
				Data: in.Data, URL: in.URL, Path: in.Path, Format: in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("get info failed: %w", err)
			}

			output, _ := json.Marshal(info)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
