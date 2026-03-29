package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type YtdlpRunner struct {
	BinaryPath    string
	CookieBrowser string
	CookieFile    string
}

func NewYtdlpRunner(binaryPath, cookieBrowser, cookieFile string) *YtdlpRunner {
	return &YtdlpRunner{
		BinaryPath:    binaryPath,
		CookieBrowser: cookieBrowser,
		CookieFile:    cookieFile,
	}
}

// cookieArgs returns cookie args. Priority: cookies.txt file > browser cookies.
func (y *YtdlpRunner) cookieArgs() []string {
	// Priority 1: cookies.txt file (always works, no DPAPI issue)
	if y.CookieFile != "" {
		if _, err := os.Stat(y.CookieFile); err == nil {
			return []string{"--cookies", y.CookieFile}
		}
	}
	// Priority 2: browser cookies (may fail with DPAPI on Windows)
	if y.CookieBrowser != "" && y.CookieBrowser != "none" {
		return []string{"--cookies-from-browser", y.CookieBrowser}
	}
	return nil
}

// GetInfo fetches video metadata without downloading.
func (y *YtdlpRunner) GetInfo(ctx context.Context, url string) (map[string]interface{}, error) {
	args := []string{"--dump-json", "--no-download", "--no-playlist"}
	args = append(args, y.cookieArgs()...)
	args = append(args, url)
	cmd := exec.CommandContext(ctx, y.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp error: %v\nstderr: %s", err, stderr.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %v", err)
	}
	return result, nil
}

// Download downloads a video with the given options.
func (y *YtdlpRunner) Download(ctx context.Context, url, quality, outputTemplate string) (string, error) {
	formatStr := qualityToFormat(quality)

	args := []string{
		"-f", formatStr,
		"--merge-output-format", "mp4",
		"--postprocessor-args", "ffmpeg:-c:v copy -c:a aac -b:a 192k",
		"--no-playlist",
		"--windows-filenames",
	}
	args = append(args, y.cookieArgs()...)
	args = append(args, "-o", outputTemplate, "--print", "after_move:filepath", url)

	cmd := exec.CommandContext(ctx, y.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp download error: %v\nstderr: %s", err, stderr.String())
	}

	// The --print flag outputs the final filepath
	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		return outputTemplate, nil
	}
	return string(output), nil
}

// DownloadAudioOnly downloads only the audio track.
func (y *YtdlpRunner) DownloadAudioOnly(ctx context.Context, url, outputTemplate string) (string, error) {
	args := []string{
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--no-playlist",
		"--windows-filenames",
	}
	args = append(args, y.cookieArgs()...)
	args = append(args, "-o", outputTemplate, "--print", "after_move:filepath", url)

	cmd := exec.CommandContext(ctx, y.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp audio download error: %v\nstderr: %s", err, stderr.String())
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		return outputTemplate, nil
	}
	return string(output), nil
}

// DownloadSubtitles downloads subtitles from a URL using yt-dlp.
// Tries manual subs first, then auto-generated subs.
// Returns the path to the downloaded .srt file.
func (y *YtdlpRunner) DownloadSubtitles(ctx context.Context, url, language, outputDir string) (string, error) {
	if language == "" || language == "auto" {
		language = "en"
	}

	// Try manual subtitles first, then auto-generated
	args := []string{
		"--skip-download",
		"--no-playlist",
		"--write-subs",
		"--write-auto-subs",
		"--sub-lang", language,
		"--convert-subs", "srt",
	}
	args = append(args, y.cookieArgs()...)
	args = append(args, "-o", filepath.Join(outputDir, "%(title)s.%(ext)s"), url)

	cmd := exec.CommandContext(ctx, y.BinaryPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp subtitle download error: %v\nstderr: %s", err, stderr.String())
	}

	// Find the .srt file in output dir
	srtPath, err := findSrtFile(outputDir, language)
	if err != nil {
		return "", fmt.Errorf("subtitles downloaded but .srt file not found: %v", err)
	}

	return srtPath, nil
}

// ListAvailableSubtitles lists all available subtitle languages for a URL.
func (y *YtdlpRunner) ListAvailableSubtitles(ctx context.Context, url string) (manual []string, auto []string, err error) {
	args := []string{"--list-subs", "--no-download", "--no-playlist"}
	args = append(args, y.cookieArgs()...)
	args = append(args, url)
	cmd := exec.CommandContext(ctx, y.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("yt-dlp error: %v", err)
	}

	output := stdout.String()
	inManual := false
	inAuto := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Available subtitles") {
			inManual = true
			inAuto = false
			continue
		}
		if strings.Contains(line, "Available automatic captions") {
			inAuto = true
			inManual = false
			continue
		}
		if line == "" || strings.HasPrefix(line, "Language") || strings.HasPrefix(line, "---") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			lang := parts[0]
			if inManual {
				manual = append(manual, lang)
			} else if inAuto {
				auto = append(auto, lang)
			}
		}
	}
	return manual, auto, nil
}

func findSrtFile(dir, language string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	// Look for .srt files matching the language
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".srt") && strings.Contains(name, language) {
			return filepath.Join(dir, name), nil
		}
	}
	// Fallback: any .srt file
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".srt") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .srt file found in %s", dir)
}

// qualityToFormat returns yt-dlp format string.
// Prefers H.264 (avc1) video + AAC/M4A audio for Windows Media Player compatibility.
// Falls back to any codec if H.264 is not available.
func qualityToFormat(quality string) string {
	switch quality {
	case "1080p", "1080":
		return "bestvideo[vcodec^=avc1][height<=1080]+bestaudio[acodec^=mp4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]"
	case "720p", "720":
		return "bestvideo[vcodec^=avc1][height<=720]+bestaudio[acodec^=mp4a]/bestvideo[height<=720]+bestaudio/best[height<=720]"
	case "480p", "480":
		return "bestvideo[vcodec^=avc1][height<=480]+bestaudio[acodec^=mp4a]/bestvideo[height<=480]+bestaudio/best[height<=480]"
	case "360p", "360":
		return "bestvideo[vcodec^=avc1][height<=360]+bestaudio[acodec^=mp4a]/bestvideo[height<=360]+bestaudio/best[height<=360]"
	case "audio-only", "audio":
		return "bestaudio"
	default: // "best"
		return "bestvideo[vcodec^=avc1]+bestaudio[acodec^=mp4a]/bestvideo[vcodec^=avc1]+bestaudio/bestvideo+bestaudio/best"
	}
}
