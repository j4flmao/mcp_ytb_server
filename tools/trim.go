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

func registerTrimVideo(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("trim_video",
		mcp.WithDescription("Trim/cut a video segment by start and end timestamps. Uses fast lossless copy when possible."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("start",
			mcp.Required(),
			mcp.Description("Start timestamp (e.g. '1:30' or '0:01:30')"),
		),
		mcp.WithString("end",
			mcp.Required(),
			mcp.Description("End timestamp (e.g. '2:45' or '0:02:45')"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional, auto-named if omitted)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		startTS := util.GetStringArg(args, "start")
		endTS := util.GetStringArg(args, "end")

		if input == "" || startTS == "" || endTS == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input, start, and end are required", "trim_video"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "trim_video"), nil
		}

		start, err := util.ParseTimestamp(startTS)
		if err != nil {
			return util.ErrorResult(util.ErrInvalidTimestamp, err.Error(), "trim_video"), nil
		}
		end, err := util.ParseTimestamp(endTS)
		if err != nil {
			return util.ErrorResult(util.ErrInvalidTimestamp, err.Error(), "trim_video"), nil
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_trimmed"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err = ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-ss", start,
			"-to", end,
			"-c", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "trim_video"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
			"start":  start,
			"end":    end,
		}
		return util.SuccessResult(result)
	})
}

func registerCutAndKeep(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("cut_and_keep",
		mcp.WithDescription("Keep multiple time ranges from a video, discard the rest, and concatenate the kept segments."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithObject("keep",
			mcp.Required(),
			mcp.Description("Array of {start, end} objects defining segments to keep"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "cut_and_keep"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "cut_and_keep"), nil
		}

		keepRaw, ok := args["keep"]
		if !ok {
			return util.ErrorResult(util.ErrInvalidInput, "keep segments are required", "cut_and_keep"), nil
		}

		keepSlice, ok := keepRaw.([]interface{})
		if !ok {
			return util.ErrorResult(util.ErrInvalidInput, "keep must be an array of {start, end} objects", "cut_and_keep"), nil
		}

		ext := filepath.Ext(input)

		// Extract each segment
		var segFiles []string
		for i, seg := range keepSlice {
			segMap, ok := seg.(map[string]interface{})
			if !ok {
				continue
			}
			startTS, _ := segMap["start"].(string)
			endTS, _ := segMap["end"].(string)

			start, err := util.ParseTimestamp(startTS)
			if err != nil {
				return util.ErrorResult(util.ErrInvalidTimestamp, err.Error(), "cut_and_keep"), nil
			}
			end, err := util.ParseTimestamp(endTS)
			if err != nil {
				return util.ErrorResult(util.ErrInvalidTimestamp, err.Error(), "cut_and_keep"), nil
			}

			segFile := filepath.Join(cfg.TempDir, fmt.Sprintf("seg_%d%s", i, ext))
			_, err = ffmpeg.RunOverwrite(ctx,
				"-i", input,
				"-ss", start,
				"-to", end,
				"-c", "copy",
				segFile,
			)
			if err != nil {
				return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "cut_and_keep"), nil
			}
			segFiles = append(segFiles, segFile)
		}

		// Build concat file
		concatFile := filepath.Join(cfg.TempDir, "concat_list.txt")
		var concatContent strings.Builder
		for _, sf := range segFiles {
			concatContent.WriteString(fmt.Sprintf("file '%s'\n", sf))
		}
		if err := os.WriteFile(concatFile, []byte(concatContent.String()), 0o644); err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "cut_and_keep"), nil
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_cut"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-f", "concat",
			"-safe", "0",
			"-i", concatFile,
			"-c", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "cut_and_keep"), nil
		}

		// Clean up temp segment files
		for _, sf := range segFiles {
			_ = os.Remove(sf)
		}
		_ = os.Remove(concatFile)

		result := map[string]interface{}{
			"status":   "ok",
			"output":   output,
			"segments": len(keepSlice),
		}
		return util.SuccessResult(result)
	})
}

func registerSplitVideo(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("split_video",
		mcp.WithDescription("Split a video into N equal parts or at specific timestamps."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithNumber("parts",
			mcp.Description("Number of equal parts to split into"),
		),
		mcp.WithObject("at",
			mcp.Description("Array of timestamps to split at (e.g. ['1:00', '2:30'])"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "split_video"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "split_video"), nil
		}

		ext := filepath.Ext(input)
		base := strings.TrimSuffix(filepath.Base(input), ext)

		duration, err := ffmpeg.GetDuration(ctx, input)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, "Cannot get video duration: "+err.Error(), "split_video"), nil
		}

		var splitPoints []float64

		if parts := util.GetIntArg(args, "parts", 0); parts > 1 {
			segDuration := duration / float64(parts)
			for i := 1; i < parts; i++ {
				splitPoints = append(splitPoints, segDuration*float64(i))
			}
		} else if atRaw, ok := args["at"]; ok {
			if atSlice, ok := atRaw.([]interface{}); ok {
				for _, ts := range atSlice {
					if tsStr, ok := ts.(string); ok {
						sec, err := util.TimestampToSeconds(tsStr)
						if err != nil {
							return util.ErrorResult(util.ErrInvalidTimestamp, err.Error(), "split_video"), nil
						}
						splitPoints = append(splitPoints, sec)
					}
				}
			}
		} else {
			return util.ErrorResult(util.ErrInvalidInput, "Either 'parts' or 'at' is required", "split_video"), nil
		}

		// Build segments: [0, sp1], [sp1, sp2], ..., [spN, duration]
		allPoints := append([]float64{0}, splitPoints...)
		allPoints = append(allPoints, duration)

		var outputs []string
		for i := 0; i < len(allPoints)-1; i++ {
			startSec := fmt.Sprintf("%.3f", allPoints[i])
			endSec := fmt.Sprintf("%.3f", allPoints[i+1])
			outFile := filepath.Join(cfg.OutputDir, fmt.Sprintf("%s_part%d%s", base, i+1, ext))

			_, err := ffmpeg.RunOverwrite(ctx,
				"-i", input,
				"-ss", startSec,
				"-to", endSec,
				"-c", "copy",
				outFile,
			)
			if err != nil {
				return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "split_video"), nil
			}
			outputs = append(outputs, outFile)
		}

		result := map[string]interface{}{
			"status":  "ok",
			"outputs": outputs,
			"parts":   len(outputs),
		}
		return util.SuccessResult(result)
	})
}
