package util

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
)

// Error codes
const (
	ErrBinaryNotFound     = "BINARY_NOT_FOUND"
	ErrDownloadFailed     = "DOWNLOAD_FAILED"
	ErrInvalidTimestamp    = "INVALID_TIMESTAMP"
	ErrWhisperNotFound    = "WHISPER_NOT_FOUND"
	ErrOutputNotWritable  = "OUTPUT_DIR_NOT_WRITABLE"
	ErrJobNotFound        = "JOB_NOT_FOUND"
	ErrFfmpegEncodeError  = "FFMPEG_ENCODE_ERROR"
	ErrInvalidInput       = "INVALID_INPUT"
	ErrFileNotFound       = "FILE_NOT_FOUND"
)

type ToolError struct {
	Error   bool   `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Tool    string `json:"tool"`
}

// ErrorResult creates an MCP error result with structured error JSON.
func ErrorResult(code, message, tool string) *mcp.CallToolResult {
	te := ToolError{
		Error:   true,
		Code:    code,
		Message: message,
		Tool:    tool,
	}
	data, _ := json.Marshal(te)
	return mcp.NewToolResultError(string(data))
}

// SuccessResult creates an MCP success result with JSON content.
func SuccessResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

// TextResult creates a simple text MCP result.
func TextResult(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

// CheckBinary verifies a binary is available in PATH.
func CheckBinary(name, path string) error {
	_, err := exec.LookPath(path)
	if err != nil {
		return fmt.Errorf("%s not found at '%s'. Please install it", name, path)
	}
	return nil
}

// GetStringArg safely extracts a string argument from the request.
func GetStringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetFloatArg safely extracts a float argument from the request.
func GetFloatArg(args map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return defaultVal
}

// GetBoolArg safely extracts a bool argument from the request.
func GetBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetIntArg safely extracts an int argument from the request.
func GetIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return defaultVal
}
