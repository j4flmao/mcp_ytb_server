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

func registerExportVideo(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("export_video",
		mcp.WithDescription("Final encode/export a video with format and quality presets optimized for YouTube, TikTok, Twitter, Discord, web, etc."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("output",
			mcp.Description("Output file path or filename"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: mp4, webm, mov, mkv, gif, mp3, aac (default: mp4)"),
		),
		mcp.WithString("preset",
			mcp.Description("Export preset: youtube, tiktok, twitter, web, discord, gif, audio_only, lossless"),
		),
		mcp.WithString("quality",
			mcp.Description("Quality: high, medium, small, custom (default: high)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "export_video"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "export_video"), nil
		}

		format := util.GetStringArg(args, "format")
		if format == "" {
			format = "mp4"
		}
		preset := util.GetStringArg(args, "preset")
		quality := util.GetStringArg(args, "quality")
		if quality == "" {
			quality = "high"
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_exported."+format)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		ffmpegArgs := buildExportArgs(input, output, format, preset, quality)

		_, err := ffmpeg.RunOverwrite(ctx, ffmpegArgs...)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "export_video"), nil
		}

		result := map[string]interface{}{
			"status":  "ok",
			"output":  output,
			"format":  format,
			"preset":  preset,
			"quality": quality,
		}
		return util.SuccessResult(result)
	})
}

func buildExportArgs(input, output, format, preset, quality string) []string {
	args := []string{"-i", input}

	switch preset {
	case "youtube":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", "18",
			"-b:v", "8M",
			"-maxrate", "12M",
			"-bufsize", "16M",
			"-c:a", "aac",
			"-b:a", "192k",
			"-movflags", "+faststart",
		)
	case "tiktok":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "23",
			"-b:v", "5M",
			"-vf", "scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
		)
	case "twitter":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "23",
			"-b:v", "3M",
			"-vf", "scale=-2:720",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
		)
	case "web":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "26",
			"-b:v", "2M",
			"-vf", "scale=-2:720",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
		)
	case "discord":
		// Discord has 8MB file size limit
		args = append(args,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", "28",
			"-b:v", "1M",
			"-vf", "scale=-2:720",
			"-c:a", "aac",
			"-b:a", "96k",
			"-movflags", "+faststart",
			"-fs", "8M",
		)
	case "gif":
		args = append(args,
			"-vf", "fps=15,scale=480:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse",
			"-loop", "0",
		)
	case "audio_only":
		if format == "mp3" {
			args = append(args,
				"-vn",
				"-c:a", "libmp3lame",
				"-b:a", "192k",
			)
		} else {
			args = append(args,
				"-vn",
				"-c:a", "aac",
				"-b:a", "192k",
			)
		}
	case "lossless":
		args = append(args,
			"-c:v", "libx264",
			"-crf", "0",
			"-preset", "ultrafast",
			"-c:a", "flac",
		)
	default:
		// Use quality setting
		switch quality {
		case "high":
			args = append(args,
				"-c:v", "libx264",
				"-preset", "slow",
				"-crf", "18",
				"-c:a", "aac",
				"-b:a", "192k",
				"-movflags", "+faststart",
			)
		case "medium":
			args = append(args,
				"-c:v", "libx264",
				"-preset", "medium",
				"-crf", "23",
				"-c:a", "aac",
				"-b:a", "128k",
				"-movflags", "+faststart",
			)
		case "small":
			args = append(args,
				"-c:v", "libx264",
				"-preset", "fast",
				"-crf", "28",
				"-c:a", "aac",
				"-b:a", "96k",
				"-movflags", "+faststart",
			)
		default:
			args = append(args,
				"-c:v", "libx264",
				"-preset", "medium",
				"-crf", "23",
				"-c:a", "aac",
				"-b:a", "128k",
			)
		}
	}

	args = append(args, output)
	return args
}
