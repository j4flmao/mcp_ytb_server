package util

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type WhisperRunner struct {
	BinaryPath string
	ModelName  string
}

func NewWhisperRunner(binaryPath, modelName string) *WhisperRunner {
	return &WhisperRunner{
		BinaryPath: binaryPath,
		ModelName:  modelName,
	}
}

// Transcribe generates an SRT subtitle file from an audio file.
// Returns the path to the generated .srt file.
func (w *WhisperRunner) Transcribe(ctx context.Context, audioPath, outputDir, language, model string) (string, error) {
	if model == "" {
		model = w.ModelName
	}
	if language == "" {
		language = "auto"
	}

	args := []string{
		"--model", model,
		"--output_format", "srt",
		"--output_dir", outputDir,
	}

	if language != "auto" {
		args = append(args, "--language", language)
	}

	args = append(args, audioPath)

	cmd := exec.CommandContext(ctx, w.BinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper error: %v\nstderr: %s", err, stderr.String())
	}

	// Whisper outputs <filename>.srt in the output directory
	base := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	srtPath := filepath.Join(outputDir, base+".srt")

	return srtPath, nil
}

// IsAvailable checks if the whisper binary is accessible.
func (w *WhisperRunner) IsAvailable() bool {
	_, err := exec.LookPath(w.BinaryPath)
	return err == nil
}
