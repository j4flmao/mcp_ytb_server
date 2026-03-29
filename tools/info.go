package tools

import (
	"context"
	"fmt"

	"video-pipeline-mcp/config"
	"video-pipeline-mcp/util"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGetVideoInfo(s *server.MCPServer, cfg *config.Config, ytdlp *util.YtdlpRunner) {
	tool := mcp.NewTool("get_video_info",
		mcp.WithDescription("Probe video metadata from a URL without downloading. Returns title, duration, resolution, available formats, and subtitle info."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("Video URL (YouTube, TikTok, Instagram, X, etc.)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url := util.GetStringArg(req.Params.Arguments, "url")
		if url == "" {
			return util.ErrorResult(util.ErrInvalidInput, "url is required", "get_video_info"), nil
		}

		if err := util.CheckBinary("yt-dlp", cfg.YtdlpPath); err != nil {
			return util.ErrorResult(util.ErrBinaryNotFound, err.Error(), "get_video_info"), nil
		}

		info, err := ytdlp.GetInfo(ctx, url)
		if err != nil {
			return util.ErrorResult(util.ErrDownloadFailed, err.Error(), "get_video_info"), nil
		}

		// Extract relevant fields
		title, _ := info["title"].(string)
		duration, _ := info["duration"].(float64)
		width, _ := info["width"].(float64)
		height, _ := info["height"].(float64)
		fps, _ := info["fps"].(float64)
		filesize, _ := info["filesize_approx"].(float64)

		// Extract available formats
		var formats []string
		if fmts, ok := info["formats"].([]interface{}); ok {
			seen := make(map[string]bool)
			for _, f := range fmts {
				if fm, ok := f.(map[string]interface{}); ok {
					if h, ok := fm["height"].(float64); ok && h > 0 {
						label := fmt.Sprintf("%dp", int(h))
						if !seen[label] {
							seen[label] = true
							formats = append(formats, label)
						}
					}
				}
			}
		}

		// Check subtitles
		hasSubtitles := false
		var subLangs []string
		if subs, ok := info["subtitles"].(map[string]interface{}); ok && len(subs) > 0 {
			hasSubtitles = true
			for lang := range subs {
				subLangs = append(subLangs, lang)
			}
		}
		if autoSubs, ok := info["automatic_captions"].(map[string]interface{}); ok && len(autoSubs) > 0 {
			hasSubtitles = true
			for lang := range autoSubs {
				subLangs = append(subLangs, lang+"(auto)")
			}
		}

		result := map[string]interface{}{
			"title":                    title,
			"duration":                 util.FormatDuration(duration),
			"duration_seconds":         int(duration),
			"resolution":              fmt.Sprintf("%dx%d", int(width), int(height)),
			"fps":                     int(fps),
			"size_mb":                 util.FormatSizeMB(filesize),
			"formats":                 formats,
			"has_subtitles":           hasSubtitles,
			"available_subtitle_langs": subLangs,
		}

		return util.SuccessResult(result)
	})
}
