package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"video-pipeline-mcp/config"
	"video-pipeline-mcp/util"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGenerateThumbnail(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("generate_thumbnail",
		mcp.WithDescription("Extract a single frame from a video as a JPEG or PNG thumbnail image."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("timestamp",
			mcp.Description("Time of the frame to extract (e.g. '0:45', default: '0:00')"),
		),
		mcp.WithString("output",
			mcp.Description("Output image filename (optional)"),
		),
		mcp.WithNumber("width",
			mcp.Description("Optional resize width in pixels (height auto-calculated)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "generate_thumbnail"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "generate_thumbnail"), nil
		}

		timestamp := util.GetStringArg(args, "timestamp")
		if timestamp == "" {
			timestamp = "0:00"
		}
		ts, err := util.ParseTimestamp(timestamp)
		if err != nil {
			return util.ErrorResult(util.ErrInvalidTimestamp, err.Error(), "generate_thumbnail"), nil
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
			output = filepath.Join(cfg.OutputDir, base+"_thumb.jpg")
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		ffmpegArgs := []string{
			"-i", input,
			"-ss", ts,
			"-vframes", "1",
		}

		width := util.GetIntArg(args, "width", 0)
		if width > 0 {
			ffmpegArgs = append(ffmpegArgs, "-vf", fmt.Sprintf("scale=%d:-1", width))
		}

		ffmpegArgs = append(ffmpegArgs, output)

		_, err = ffmpeg.RunOverwrite(ctx, ffmpegArgs...)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "generate_thumbnail"), nil
		}

		result := map[string]interface{}{
			"status":    "ok",
			"output":    output,
			"timestamp": timestamp,
		}
		return util.SuccessResult(result)
	})
}

func registerGetFileInfo(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("get_file_info",
		mcp.WithDescription("Show detailed codec, bitrate, resolution, and stream information for any local video/audio file."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to the file to inspect"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "get_file_info"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "get_file_info"), nil
		}

		info, err := ffmpeg.ProbeJSON(ctx, input)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "get_file_info"), nil
		}

		// Extract format info
		formatInfo := map[string]interface{}{}
		if format, ok := info["format"].(map[string]interface{}); ok {
			formatInfo["filename"] = format["filename"]
			formatInfo["format"] = format["format_long_name"]
			formatInfo["duration"] = format["duration"]
			formatInfo["size_bytes"] = format["size"]
			formatInfo["bit_rate"] = format["bit_rate"]
		}

		// Extract stream info
		var streams []map[string]interface{}
		if streamList, ok := info["streams"].([]interface{}); ok {
			for _, s := range streamList {
				if stream, ok := s.(map[string]interface{}); ok {
					streamInfo := map[string]interface{}{
						"type":       stream["codec_type"],
						"codec":      stream["codec_name"],
						"codec_long": stream["codec_long_name"],
					}
					if stream["codec_type"] == "video" {
						streamInfo["width"] = stream["width"]
						streamInfo["height"] = stream["height"]
						streamInfo["fps"] = stream["r_frame_rate"]
						streamInfo["pix_fmt"] = stream["pix_fmt"]
					}
					if stream["codec_type"] == "audio" {
						streamInfo["sample_rate"] = stream["sample_rate"]
						streamInfo["channels"] = stream["channels"]
						streamInfo["bit_rate"] = stream["bit_rate"]
					}
					streams = append(streams, streamInfo)
				}
			}
		}

		result := map[string]interface{}{
			"format":  formatInfo,
			"streams": streams,
		}
		return util.SuccessResult(result)
	})
}
