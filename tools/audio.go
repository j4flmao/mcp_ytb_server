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

func registerMuteAudio(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("mute_audio",
		mcp.WithDescription("Remove or mute the audio track from a video."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "mute_audio"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "mute_audio"), nil
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_muted"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-an",
			"-c:v", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "mute_audio"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
		}
		return util.SuccessResult(result)
	})
}

func registerReplaceAudio(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("replace_audio",
		mcp.WithDescription("Replace the audio track in a video with another audio file. Optionally mix/overlay instead of replace."),
		mcp.WithString("video",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("audio",
			mcp.Required(),
			mcp.Description("Path to audio file to use"),
		),
		mcp.WithBoolean("mix",
			mcp.Description("true = overlay on original audio, false = replace (default: false)"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		video := util.GetStringArg(args, "video")
		audio := util.GetStringArg(args, "audio")
		mix := util.GetBoolArg(args, "mix", false)

		if video == "" || audio == "" {
			return util.ErrorResult(util.ErrInvalidInput, "video and audio are required", "replace_audio"), nil
		}

		for _, f := range []string{video, audio} {
			if _, err := os.Stat(f); os.IsNotExist(err) {
				return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", f), "replace_audio"), nil
			}
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(video)
			base := strings.TrimSuffix(filepath.Base(video), ext)
			output = filepath.Join(cfg.OutputDir, base+"_newaudio"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		var err error
		if mix {
			_, err = ffmpeg.RunOverwrite(ctx,
				"-i", video,
				"-i", audio,
				"-filter_complex", "[0:a][1:a]amix=inputs=2:duration=first[a]",
				"-map", "0:v",
				"-map", "[a]",
				"-c:v", "copy",
				output,
			)
		} else {
			_, err = ffmpeg.RunOverwrite(ctx,
				"-i", video,
				"-i", audio,
				"-map", "0:v",
				"-map", "1:a",
				"-c:v", "copy",
				"-shortest",
				output,
			)
		}

		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "replace_audio"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
			"mode":   map[bool]string{true: "mixed", false: "replaced"}[mix],
		}
		return util.SuccessResult(result)
	})
}

func registerNormalizeAudio(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("normalize_audio",
		mcp.WithDescription("Normalize audio volume to EBU R128 broadcast standard."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video/audio file"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "normalize_audio"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "normalize_audio"), nil
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_normalized"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-af", "loudnorm=I=-16:LRA=11:TP=-1.5",
			"-c:v", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "normalize_audio"), nil
		}

		result := map[string]interface{}{
			"status":  "ok",
			"output":  output,
			"message": "Audio normalized to EBU R128 (I=-16 LUFS, LRA=11, TP=-1.5 dBTP)",
		}
		return util.SuccessResult(result)
	})
}
