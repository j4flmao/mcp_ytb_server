package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseTimestamp converts "MM:SS" or "HH:MM:SS" to "HH:MM:SS" format for ffmpeg.
func ParseTimestamp(ts string) (string, error) {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return "00:00:00", nil
	}

	// Already in HH:MM:SS format
	if matched, _ := regexp.MatchString(`^\d+:\d{2}:\d{2}$`, ts); matched {
		return ts, nil
	}

	// MM:SS format
	if matched, _ := regexp.MatchString(`^\d+:\d{2}$`, ts); matched {
		return "00:" + ts, nil
	}

	// Seconds only
	if matched, _ := regexp.MatchString(`^\d+(\.\d+)?$`, ts); matched {
		sec, err := strconv.ParseFloat(ts, 64)
		if err != nil {
			return "", fmt.Errorf("invalid timestamp: %s", ts)
		}
		h := int(sec) / 3600
		m := (int(sec) % 3600) / 60
		s := int(sec) % 60
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s), nil
	}

	return "", fmt.Errorf("invalid timestamp format: %s (use MM:SS or HH:MM:SS)", ts)
}

// TimestampToSeconds converts a timestamp string to seconds.
func TimestampToSeconds(ts string) (float64, error) {
	ts = strings.TrimSpace(ts)
	parts := strings.Split(ts, ":")
	switch len(parts) {
	case 1:
		return strconv.ParseFloat(parts[0], 64)
	case 2:
		m, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		s, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		return m*60 + s, nil
	case 3:
		h, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		m, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		s, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, err
		}
		return h*3600 + m*60 + s, nil
	}
	return 0, fmt.Errorf("invalid timestamp: %s", ts)
}

// FormatDuration formats seconds into "M:SS" human-readable format.
func FormatDuration(seconds float64) string {
	total := int(seconds)
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// FormatSizeMB formats bytes to megabytes.
func FormatSizeMB(bytes float64) float64 {
	return float64(int(bytes/1024/1024*10)) / 10
}

// SanitizeFilename removes characters that are invalid in filenames.
func SanitizeFilename(name string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	name = re.ReplaceAllString(name, "_")
	if len(name) > 200 {
		name = name[:200]
	}
	return strings.TrimSpace(name)
}
