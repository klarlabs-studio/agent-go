// Package image provides image processing tools for agent-go.
//
// This pack includes tools for image operations:
//   - image_resize: Resize an image to specified dimensions
//   - image_crop: Crop an image to specified region
//   - image_convert: Convert image format (PNG, JPEG, WebP, etc.)
//   - image_compress: Compress image to reduce file size
//   - image_rotate: Rotate an image by degrees
//   - image_thumbnail: Generate a thumbnail
//   - image_metadata: Extract image metadata (EXIF, dimensions)
//   - image_watermark: Add a watermark to an image
//
// Supports common formats: PNG, JPEG, GIF, WebP, TIFF, BMP.
package image

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the image processing tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("image").
		WithDescription("Image processing and manipulation tools").
		WithVersion("0.1.0").
		AddTools(
			imageResize(),
			imageCrop(),
			imageConvert(),
			imageCompress(),
			imageRotate(),
			imageThumbnail(),
			imageMetadata(),
			imageWatermark(),
		).
		AllowInState(agent.StateExplore, "image_metadata").
		AllowInState(agent.StateAct, "image_resize", "image_crop", "image_convert", "image_compress", "image_rotate", "image_thumbnail", "image_metadata", "image_watermark").
		AllowInState(agent.StateValidate, "image_metadata").
		Build()
}

func imageResize() tool.Tool {
	return tool.NewBuilder("image_resize").
		WithDescription("Resize an image to specified dimensions").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func imageCrop() tool.Tool {
	return tool.NewBuilder("image_crop").
		WithDescription("Crop an image to a specified region").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func imageConvert() tool.Tool {
	return tool.NewBuilder("image_convert").
		WithDescription("Convert image to a different format").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func imageCompress() tool.Tool {
	return tool.NewBuilder("image_compress").
		WithDescription("Compress an image to reduce file size").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func imageRotate() tool.Tool {
	return tool.NewBuilder("image_rotate").
		WithDescription("Rotate an image by specified degrees").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func imageThumbnail() tool.Tool {
	return tool.NewBuilder("image_thumbnail").
		WithDescription("Generate a thumbnail for an image").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func imageMetadata() tool.Tool {
	return tool.NewBuilder("image_metadata").
		WithDescription("Extract metadata from an image (EXIF, dimensions, format)").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func imageWatermark() tool.Tool {
	return tool.NewBuilder("image_watermark").
		WithDescription("Add a watermark to an image").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}
