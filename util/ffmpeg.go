package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type FfmpegRunner struct {
	FfmpegPath  string
	FfprobePath string
}

func NewFfmpegRunner(ffmpegPath, ffprobePath string) *FfmpegRunner {
	return &FfmpegRunner{
		FfmpegPath:  ffmpegPath,
		FfprobePath: ffprobePath,
	}
}

// Run executes an ffmpeg command with the given arguments.
func (f *FfmpegRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, f.FfmpegPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %v\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// RunWithArgs executes ffmpeg with -y (overwrite) prepended.
func (f *FfmpegRunner) RunOverwrite(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{"-y"}, args...)
	return f.Run(ctx, fullArgs...)
}

// ProbeJSON runs ffprobe and returns the parsed JSON output.
func (f *FfmpegRunner) ProbeJSON(ctx context.Context, input string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, f.FfprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		input,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe error: %v\nstderr: %s", err, stderr.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %v", err)
	}
	return result, nil
}

// GetDuration returns the duration in seconds from ffprobe.
func (f *FfmpegRunner) GetDuration(ctx context.Context, input string) (float64, error) {
	cmd := exec.CommandContext(ctx, f.FfprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0, err
	}
	var dur float64
	_, err := fmt.Sscanf(strings.TrimSpace(stdout.String()), "%f", &dur)
	return dur, err
}

// ExtractAudio extracts audio from a video file as WAV (16kHz mono for Whisper).
func (f *FfmpegRunner) ExtractAudio(ctx context.Context, input, output string) error {
	_, err := f.RunOverwrite(ctx,
		"-i", input,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		output,
	)
	return err
}
