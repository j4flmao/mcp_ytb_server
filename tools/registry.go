package tools

import (
	"video-pipeline-mcp/config"
	"video-pipeline-mcp/jobs"
	"video-pipeline-mcp/util"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Register registers all MCP tools on the server.
func Register(s *server.MCPServer, cfg *config.Config, queue *jobs.Queue) {
	ffmpeg := util.NewFfmpegRunner(cfg.FfmpegPath, cfg.FfprobePath)
	ytdlp := util.NewYtdlpRunner(cfg.YtdlpPath, cfg.CookieBrowser, cfg.CookieFile)
	whisper := util.NewWhisperRunner(cfg.WhisperPath, cfg.WhisperModel)

	// Phase 2 — Download & Inspect (synchronous)
	registerGetVideoInfo(s, cfg, ytdlp)
	registerDownloadVideo(s, cfg, ytdlp)
	registerSetOutputPath(s, cfg)

	// Phase 3 — Edit & Trim (synchronous)
	registerTrimVideo(s, cfg, ffmpeg)
	registerCutAndKeep(s, cfg, ffmpeg)
	registerMergeClips(s, cfg, ffmpeg)
	registerSplitVideo(s, cfg, ffmpeg)
	registerMuteAudio(s, cfg, ffmpeg)
	registerReplaceAudio(s, cfg, ffmpeg)
	registerNormalizeAudio(s, cfg, ffmpeg)

	// Phase 4 — Subtitles (synchronous)
	registerGenerateSubtitles(s, cfg, ffmpeg, whisper)
	registerDownloadSubtitles(s, cfg, ytdlp)
	registerListSubtitles(s, cfg, ytdlp)
	registerAddSubtitleFile(s, cfg, ffmpeg)
	registerRemoveSubtitles(s, cfg, ffmpeg)

	// Phase 5 — Effects & Overlays (synchronous)
	registerAddFade(s, cfg, ffmpeg)
	registerAddWatermark(s, cfg, ffmpeg)
	registerBlurRegion(s, cfg, ffmpeg)
	registerZoomEffect(s, cfg, ffmpeg)
	registerColorGrade(s, cfg, ffmpeg)
	registerSpeedChange(s, cfg, ffmpeg)
	registerAddIntroOutro(s, cfg, ffmpeg)

	// Phase 6 — Export & Finalize (synchronous)
	registerExportVideo(s, cfg, ffmpeg)
	registerGenerateThumbnail(s, cfg, ffmpeg)
	registerGetFileInfo(s, cfg, ffmpeg)

	// Phase 7 — Job Queue (for status tracking)
	registerListJobs(s, queue)
	registerGetJobStatus(s, queue)
	registerCancelJob(s, queue)
}

// helper to add a tool to the server
func addTool(s *server.MCPServer, tool mcp.Tool, handler server.ToolHandlerFunc) {
	s.AddTool(tool, handler)
}
