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

func registerGenerateSubtitles(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner, whisper *util.WhisperRunner) {
	tool := mcp.NewTool("generate_subtitles",
		mcp.WithDescription("Auto-generate subtitles for a video. First tries downloading existing subtitles from the source URL (YouTube, TikTok, etc.) via yt-dlp — no extra software needed. Falls back to local Whisper if installed. Supports Vietnamese, English, and 50+ languages."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("url",
			mcp.Description("Original video URL — if provided, will try to download existing subtitles from the platform first (fastest method, no Whisper needed)"),
		),
		mcp.WithString("language",
			mcp.Description("Language code: vi, en, auto, or any ISO 639-1 code (default: vi)"),
		),
		mcp.WithString("model",
			mcp.Description("Whisper model: tiny, base, small, medium, large (only used if Whisper fallback is needed)"),
		),
		mcp.WithBoolean("burn_in",
			mcp.Description("true = bake subtitles into video, false = save as .srt only (default: false)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "generate_subtitles"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "generate_subtitles"), nil
		}

		url := util.GetStringArg(args, "url")
		language := util.GetStringArg(args, "language")
		if language == "" {
			language = "vi"
		}
		model := util.GetStringArg(args, "model")
		if model == "" {
			model = cfg.WhisperModel
		}
		burnIn := util.GetBoolArg(args, "burn_in", false)

		ytdlp := util.NewYtdlpRunner(cfg.YtdlpPath, cfg.CookieBrowser, cfg.CookieFile)

		ext := filepath.Ext(input)
		base := strings.TrimSuffix(filepath.Base(input), ext)
		finalSrt := filepath.Join(cfg.OutputDir, base+"."+language+".srt")

		var srtPath string
		var err error

		// === Method 1: Download subs from platform via yt-dlp (fastest) ===
		if url != "" {
			subDir := filepath.Join(cfg.TempDir, "subs_"+base)
			_ = os.MkdirAll(subDir, 0o755)
			srtPath, err = ytdlp.DownloadSubtitles(ctx, url, language, subDir)
			if err == nil {
				data, err2 := os.ReadFile(srtPath)
				if err2 == nil {
					_ = os.WriteFile(finalSrt, data, 0o644)
					srtPath = finalSrt
				}
				_ = os.RemoveAll(subDir)
			}
		}

		// === Method 2: Local Whisper fallback ===
		if srtPath == "" && whisper.IsAvailable() {
			audioPath := filepath.Join(cfg.TempDir, "whisper_audio.wav")
			if err2 := ffmpeg.ExtractAudio(ctx, input, audioPath); err2 != nil {
				return util.ErrorResult(util.ErrFfmpegEncodeError, fmt.Sprintf("Failed to extract audio: %v", err2), "generate_subtitles"), nil
			}
			defer os.Remove(audioPath)

			srtPath, err = whisper.Transcribe(ctx, audioPath, cfg.TempDir, language, model)
			if err != nil {
				return util.ErrorResult(util.ErrFfmpegEncodeError, fmt.Sprintf("Whisper transcription failed: %v", err), "generate_subtitles"), nil
			}

			if data, err2 := os.ReadFile(srtPath); err2 == nil {
				_ = os.WriteFile(finalSrt, data, 0o644)
				srtPath = finalSrt
			}
		}

		// === No method worked ===
		if srtPath == "" {
			if url == "" {
				return util.ErrorResult(util.ErrInvalidInput, "No subtitle source available. Provide the original video URL so yt-dlp can download existing subtitles, or install Whisper for local transcription", "generate_subtitles"), nil
			}
			return util.ErrorResult(util.ErrDownloadFailed, fmt.Sprintf("No subtitles found for language '%s' on this platform, and Whisper is not installed for local transcription", language), "generate_subtitles"), nil
		}

		if !burnIn {
			result := map[string]interface{}{
				"status":   "ok",
				"output":   srtPath,
				"language": language,
				"message":  fmt.Sprintf("Subtitles saved: %s", srtPath),
			}
			return util.SuccessResult(result)
		}

		// Burn subtitles into video
		burnedOutput := filepath.Join(cfg.OutputDir, base+"_subbed"+ext)
		escapedSrt := strings.ReplaceAll(srtPath, "\\", "/")
		escapedSrt = strings.ReplaceAll(escapedSrt, ":", "\\:")

		_, err = ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-vf", fmt.Sprintf("subtitles='%s':force_style='FontName=Arial,FontSize=20,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,BorderStyle=3'", escapedSrt),
			"-c:a", "copy",
			burnedOutput,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, fmt.Sprintf("Failed to burn subtitles: %v", err), "generate_subtitles"), nil
		}

		result := map[string]interface{}{
			"status":   "ok",
			"output":   burnedOutput,
			"srt_file": srtPath,
			"language": language,
			"burn_in":  true,
			"message":  fmt.Sprintf("Subtitles burned into video: %s", burnedOutput),
		}
		return util.SuccessResult(result)
	})
}

func registerDownloadSubtitles(s *server.MCPServer, cfg *config.Config, ytdlp *util.YtdlpRunner) {
	tool := mcp.NewTool("download_subtitles",
		mcp.WithDescription("Download existing subtitles from YouTube, TikTok, or other platforms. No Whisper needed — uses yt-dlp to grab subtitles directly from the platform."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("Video URL to download subtitles from"),
		),
		mcp.WithString("language",
			mcp.Description("Language code: vi, en, or any ISO 639-1 code (default: vi)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		url := util.GetStringArg(args, "url")
		if url == "" {
			return util.ErrorResult(util.ErrInvalidInput, "url is required", "download_subtitles"), nil
		}

		language := util.GetStringArg(args, "language")
		if language == "" {
			language = "vi"
		}

		if err := util.CheckBinary("yt-dlp", cfg.YtdlpPath); err != nil {
			return util.ErrorResult(util.ErrBinaryNotFound, err.Error(), "download_subtitles"), nil
		}

		subDir := filepath.Join(cfg.TempDir, "subs_dl")
		_ = os.MkdirAll(subDir, 0o755)

		srtPath, err := ytdlp.DownloadSubtitles(ctx, url, language, subDir)
		if err != nil {
			return util.ErrorResult(util.ErrDownloadFailed,
				fmt.Sprintf("No subtitles found for language '%s'. Try a different language or check if the video has subtitles.", language),
				"download_subtitles"), nil
		}

		finalName := filepath.Base(srtPath)
		finalPath := filepath.Join(cfg.OutputDir, finalName)
		if data, err := os.ReadFile(srtPath); err == nil {
			_ = os.WriteFile(finalPath, data, 0o644)
		}
		_ = os.RemoveAll(subDir)

		result := map[string]interface{}{
			"status":   "ok",
			"output":   finalPath,
			"language": language,
			"message":  fmt.Sprintf("Subtitles downloaded: %s", finalPath),
		}
		return util.SuccessResult(result)
	})
}

func registerListSubtitles(s *server.MCPServer, cfg *config.Config, ytdlp *util.YtdlpRunner) {
	tool := mcp.NewTool("list_subtitles",
		mcp.WithDescription("List all available subtitle languages for a video URL. Shows both manual and auto-generated subtitles."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("Video URL to check available subtitles"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url := util.GetStringArg(req.Params.Arguments, "url")
		if url == "" {
			return util.ErrorResult(util.ErrInvalidInput, "url is required", "list_subtitles"), nil
		}

		manual, auto, err := ytdlp.ListAvailableSubtitles(ctx, url)
		if err != nil {
			return util.ErrorResult(util.ErrDownloadFailed, err.Error(), "list_subtitles"), nil
		}

		result := map[string]interface{}{
			"manual_subtitles":   manual,
			"auto_gen_subtitles": auto,
			"has_manual":         len(manual) > 0,
			"has_auto":           len(auto) > 0,
		}
		return util.SuccessResult(result)
	})
}

func registerAddSubtitleFile(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("add_subtitle_file",
		mcp.WithDescription("Add an existing .srt subtitle file to a video. Can burn in (hardcode) or soft-attach."),
		mcp.WithString("video",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("srt",
			mcp.Required(),
			mcp.Description("Path to .srt subtitle file"),
		),
		mcp.WithBoolean("burn_in",
			mcp.Description("true = hardcode into video, false = soft-attach as subtitle track (default: true)"),
		),
		mcp.WithObject("style",
			mcp.Description("Subtitle style options: font, size, color, outline, position"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		video := util.GetStringArg(args, "video")
		srt := util.GetStringArg(args, "srt")

		if video == "" || srt == "" {
			return util.ErrorResult(util.ErrInvalidInput, "video and srt are required", "add_subtitle_file"), nil
		}

		for _, f := range []string{video, srt} {
			if _, err := os.Stat(f); os.IsNotExist(err) {
				return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", f), "add_subtitle_file"), nil
			}
		}

		burnIn := util.GetBoolArg(args, "burn_in", true)

		ext := filepath.Ext(video)
		base := strings.TrimSuffix(filepath.Base(video), ext)
		output := filepath.Join(cfg.OutputDir, base+"_subbed"+ext)

		if burnIn {
			fontName := "Arial"
			fontSize := 22
			fontColor := "&HFFFFFF"
			if styleRaw, ok := args["style"]; ok {
				if style, ok := styleRaw.(map[string]interface{}); ok {
					if f, ok := style["font"].(string); ok {
						fontName = f
					}
					if s, ok := style["size"].(float64); ok {
						fontSize = int(s)
					}
					if c, ok := style["color"].(string); ok {
						switch c {
						case "white":
							fontColor = "&HFFFFFF"
						case "yellow":
							fontColor = "&H00FFFF"
						case "red":
							fontColor = "&H0000FF"
						}
					}
				}
			}

			escapedSrt := strings.ReplaceAll(srt, "\\", "/")
			escapedSrt = strings.ReplaceAll(escapedSrt, ":", "\\:")

			styleStr := fmt.Sprintf("FontName=%s,FontSize=%d,PrimaryColour=%s,OutlineColour=&H000000,BorderStyle=3",
				fontName, fontSize, fontColor)

			_, err := ffmpeg.RunOverwrite(ctx,
				"-i", video,
				"-vf", fmt.Sprintf("subtitles='%s':force_style='%s'", escapedSrt, styleStr),
				"-c:a", "copy",
				output,
			)
			if err != nil {
				return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "add_subtitle_file"), nil
			}
		} else {
			_, err := ffmpeg.RunOverwrite(ctx,
				"-i", video,
				"-i", srt,
				"-map", "0:v",
				"-map", "0:a?",
				"-map", "1:s",
				"-c:v", "copy",
				"-c:a", "copy",
				"-c:s", "mov_text",
				output,
			)
			if err != nil {
				return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "add_subtitle_file"), nil
			}
		}

		result := map[string]interface{}{
			"status":  "ok",
			"output":  output,
			"burn_in": burnIn,
		}
		return util.SuccessResult(result)
	})
}

func registerRemoveSubtitles(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("remove_subtitles",
		mcp.WithDescription("Remove all subtitle tracks from a video file."),
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
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "remove_subtitles"), nil
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "remove_subtitles"), nil
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_nosub"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-map", "0:v",
			"-map", "0:a?",
			"-c", "copy",
			"-sn",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "remove_subtitles"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
		}
		return util.SuccessResult(result)
	})
}
