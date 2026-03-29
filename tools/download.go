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

func registerDownloadVideo(s *server.MCPServer, cfg *config.Config, ytdlp *util.YtdlpRunner) {
	tool := mcp.NewTool("download_video",
		mcp.WithDescription("Download a video from YouTube, TikTok, Instagram, X, or 1000+ supported sites. Output path and filename are optional — defaults to the configured output directory. User can specify any path on their machine like 'D:\\Videos', '/Users/me/Desktop', etc."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("Video URL to download"),
		),
		mcp.WithString("quality",
			mcp.Description("Quality: best, 1080p, 720p, 480p, 360p, audio-only (default: best)"),
		),
		mcp.WithString("output_path",
			mcp.Description("Custom output directory — any valid path on the user's machine. Optional, uses default output dir if omitted. Examples: 'D:\\Videos', 'C:\\Users\\Hi\\Desktop', '/tmp/videos'"),
		),
		mcp.WithString("filename",
			mcp.Description("Custom filename without extension (optional, uses video title if omitted)"),
		),
		mcp.WithString("browser",
			mcp.Description("Optional: where yt-dlp should read cookies from. Examples: chrome, firefox, edge, chromium, brave, opera, vivaldi, safari, none. If omitted, uses server config (VIDEO_MCP_COOKIE_BROWSER)."),
		),
		mcp.WithString("cookie_file",
			mcp.Description("Optional: path to a Netscape-format cookies.txt file to pass to yt-dlp via --cookies. This takes priority over browser cookies. If omitted, uses server config (VIDEO_MCP_COOKIE_FILE)."),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		url := util.GetStringArg(args, "url")
		if url == "" {
			return util.ErrorResult(util.ErrInvalidInput, "url is required", "download_video"), nil
		}

		if err := util.CheckBinary("yt-dlp", cfg.YtdlpPath); err != nil {
			return util.ErrorResult(util.ErrBinaryNotFound, err.Error(), "download_video"), nil
		}

		quality := util.GetStringArg(args, "quality")
		if quality == "" {
			quality = cfg.DefaultQuality
		}

		outputPath := util.GetStringArg(args, "output_path")
		if outputPath == "" {
			outputPath = cfg.OutputDir
		}
		_ = os.MkdirAll(outputPath, 0o755)

		filename := util.GetStringArg(args, "filename")
		var outputTemplate string
		if filename != "" {
			outputTemplate = filepath.Join(outputPath, util.SanitizeFilename(filename)+".%(ext)s")
		} else {
			outputTemplate = filepath.Join(outputPath, "%(title)s.%(ext)s")
		}

		cookieFile := strings.TrimSpace(util.GetStringArg(args, "cookie_file"))
		if cookieFile == "" {
			cookieFile = strings.TrimSpace(util.GetStringArg(args, "cookies"))
		}
		if cookieFile == "" {
			cookieFile = strings.TrimSpace(cfg.CookieFile)
		}
		if cookieFile != "" && !filepath.IsAbs(cookieFile) {
			if wd, err := os.Getwd(); err == nil {
				cookieFile = filepath.Join(wd, cookieFile)
			}
		}
		if cookieFile != "" {
			if _, err := os.Stat(cookieFile); err != nil {
				return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("cookies.txt not found: %s (set VIDEO_MCP_COOKIE_FILE to an absolute path, or pass cookie_file)", cookieFile), "download_video"), nil
			}
		}

		browser := strings.TrimSpace(util.GetStringArg(args, "browser"))
		if browser == "" {
			browser = strings.TrimSpace(util.GetStringArg(args, "cookie_browser"))
		}
		if browser == "" {
			browser = strings.TrimSpace(cfg.CookieBrowser)
		}
		browser = strings.ToLower(browser)
		runner := ytdlp
		if browser != "" || cookieFile != "" {
			local := *ytdlp
			if browser != "" {
				local.CookieBrowser = browser
			}
			if cookieFile != "" {
				local.CookieFile = cookieFile
			}
			runner = &local
		}

		// Run synchronously — Claude waits for the result
		var filePath string
		var err error
		if quality == "audio-only" || quality == "audio" {
			filePath, err = runner.DownloadAudioOnly(ctx, url, outputTemplate)
		} else {
			filePath, err = runner.Download(ctx, url, quality, outputTemplate)
		}

		if err != nil {
			return util.ErrorResult(util.ErrDownloadFailed, err.Error(), "download_video"), nil
		}

		result := map[string]interface{}{
			"status":      "ok",
			"file_path":   filePath,
			"url":         url,
			"quality":     quality,
			"output_dir":  outputPath,
			"browser":     browser,
			"cookie_file": cookieFile,
			"message":     fmt.Sprintf("Downloaded successfully: %s", filePath),
		}

		return util.SuccessResult(result)
	})
}

func registerSetOutputPath(s *server.MCPServer, cfg *config.Config) {
	tool := mcp.NewTool("set_output_path",
		mcp.WithDescription("Set or change the default output directory for all video operations."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("New output directory path"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		newPath := util.GetStringArg(req.Params.Arguments, "path")
		if newPath == "" {
			return util.ErrorResult(util.ErrInvalidInput, "path is required", "set_output_path"), nil
		}

		if err := os.MkdirAll(newPath, 0o755); err != nil {
			return util.ErrorResult(util.ErrOutputNotWritable,
				fmt.Sprintf("Cannot create directory '%s': %v", newPath, err),
				"set_output_path"), nil
		}

		cfg.OutputDir = newPath

		result := map[string]interface{}{
			"status":     "ok",
			"output_dir": newPath,
			"message":    fmt.Sprintf("Output directory set to: %s", newPath),
		}

		return util.SuccessResult(result)
	})
}
