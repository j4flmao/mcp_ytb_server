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

func registerMergeClips(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("merge_clips",
		mcp.WithDescription("Join/concatenate multiple video files into one in order."),
		mcp.WithObject("inputs",
			mcp.Required(),
			mcp.Description("Array of file paths to merge in order"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments

		inputsRaw, ok := args["inputs"]
		if !ok {
			return util.ErrorResult(util.ErrInvalidInput, "inputs array is required", "merge_clips"), nil
		}

		inputSlice, ok := inputsRaw.([]interface{})
		if !ok || len(inputSlice) < 2 {
			return util.ErrorResult(util.ErrInvalidInput, "inputs must be an array of at least 2 file paths", "merge_clips"), nil
		}

		var inputs []string
		for _, v := range inputSlice {
			if s, ok := v.(string); ok {
				if _, err := os.Stat(s); os.IsNotExist(err) {
					return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", s), "merge_clips"), nil
				}
				inputs = append(inputs, s)
			}
		}

		// Build concat file
		concatFile := filepath.Join(cfg.TempDir, "merge_list.txt")
		var content strings.Builder
		for _, f := range inputs {
			content.WriteString(fmt.Sprintf("file '%s'\n", f))
		}
		if err := os.WriteFile(concatFile, []byte(content.String()), 0o644); err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "merge_clips"), nil
		}
		defer os.Remove(concatFile)

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(inputs[0])
			output = filepath.Join(cfg.OutputDir, "merged"+ext)
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
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "merge_clips"), nil
		}

		result := map[string]interface{}{
			"status":      "ok",
			"output":      output,
			"clips_count": len(inputs),
		}
		return util.SuccessResult(result)
	})
}
