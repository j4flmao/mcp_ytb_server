package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

type Config struct {
	OutputDir      string `json:"output_dir"`
	TempDir        string `json:"temp_dir"`
	YtdlpPath     string `json:"ytdlp_path"`
	FfmpegPath    string `json:"ffmpeg_path"`
	FfprobePath   string `json:"ffprobe_path"`
	WhisperPath   string `json:"whisper_path"`
	WhisperModel  string `json:"whisper_model"`
	MaxConcurrent  int    `json:"max_concurrent"`
	DefaultQuality string `json:"default_quality"`
	CookieBrowser  string `json:"cookie_browser"`
	CookieFile     string `json:"cookie_file"`
}

func defaultOutputDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads", "video-mcp-output")
}

func defaultTempDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "video-mcp")
	}
	return "/tmp/video-mcp"
}

func Load() *Config {
	cfg := &Config{
		OutputDir:      defaultOutputDir(),
		TempDir:        defaultTempDir(),
		YtdlpPath:     "yt-dlp",
		FfmpegPath:    "ffmpeg",
		FfprobePath:   "ffprobe",
		WhisperPath:   "whisper",
		WhisperModel:  "medium",
		MaxConcurrent:  2,
		DefaultQuality: "best",
		CookieBrowser:  "",
	}

	// Override from environment variables
	if v := os.Getenv("VIDEO_MCP_OUTPUT_DIR"); v != "" {
		cfg.OutputDir = v
	}
	if v := os.Getenv("VIDEO_MCP_TEMP_DIR"); v != "" {
		cfg.TempDir = v
	}
	if v := os.Getenv("VIDEO_MCP_YTDLP_PATH"); v != "" {
		cfg.YtdlpPath = v
	}
	if v := os.Getenv("VIDEO_MCP_FFMPEG_PATH"); v != "" {
		cfg.FfmpegPath = v
	}
	if v := os.Getenv("VIDEO_MCP_FFPROBE_PATH"); v != "" {
		cfg.FfprobePath = v
	}
	if v := os.Getenv("VIDEO_MCP_WHISPER_PATH"); v != "" {
		cfg.WhisperPath = v
	}
	if v := os.Getenv("VIDEO_MCP_WHISPER_MODEL"); v != "" {
		cfg.WhisperModel = v
	}
	if v := os.Getenv("VIDEO_MCP_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxConcurrent = n
		}
	}
	if v := os.Getenv("VIDEO_MCP_QUALITY"); v != "" {
		cfg.DefaultQuality = v
	}
	if v := os.Getenv("VIDEO_MCP_COOKIE_BROWSER"); v != "" {
		cfg.CookieBrowser = v
	}
	if v := os.Getenv("VIDEO_MCP_COOKIE_FILE"); v != "" {
		cfg.CookieFile = v
	}

	// Try loading from config file
	configPath := configFilePath()
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	// Ensure directories exist
	_ = os.MkdirAll(cfg.OutputDir, 0o755)
	_ = os.MkdirAll(cfg.TempDir, 0o755)

	return cfg
}

func configFilePath() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(home, ".config", "video-mcp", "config.json")
	}
	return filepath.Join(home, ".config", "video-mcp", "config.json")
}

func (c *Config) Save() error {
	path := configFilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
