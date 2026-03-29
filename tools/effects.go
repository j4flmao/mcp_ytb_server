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

func registerAddFade(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("add_fade",
		mcp.WithDescription("Add fade in and/or fade out effect to video and audio."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithNumber("fade_in",
			mcp.Description("Fade in duration in seconds (0 to disable, default: 0)"),
		),
		mcp.WithNumber("fade_out",
			mcp.Description("Fade out duration in seconds (0 to disable, default: 0)"),
		),
		mcp.WithBoolean("fade_audio",
			mcp.Description("Also apply fade to audio (default: true)"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "add_fade"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "add_fade"), nil
		}

		fadeIn := util.GetFloatArg(args, "fade_in", 0)
		fadeOut := util.GetFloatArg(args, "fade_out", 0)
		fadeAudio := util.GetBoolArg(args, "fade_audio", true)

		if fadeIn == 0 && fadeOut == 0 {
			return util.ErrorResult(util.ErrInvalidInput, "At least one of fade_in or fade_out must be > 0", "add_fade"), nil
		}

		duration, err := ffmpeg.GetDuration(ctx, input)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, "Cannot get duration: "+err.Error(), "add_fade"), nil
		}

		// Build video filter
		var vFilters []string
		if fadeIn > 0 {
			vFilters = append(vFilters, fmt.Sprintf("fade=t=in:st=0:d=%.2f", fadeIn))
		}
		if fadeOut > 0 {
			fadeOutStart := duration - fadeOut
			if fadeOutStart < 0 {
				fadeOutStart = 0
			}
			vFilters = append(vFilters, fmt.Sprintf("fade=t=out:st=%.2f:d=%.2f", fadeOutStart, fadeOut))
		}

		ffmpegArgs := []string{"-i", input}

		ffmpegArgs = append(ffmpegArgs, "-vf", strings.Join(vFilters, ","))

		if fadeAudio {
			var aFilters []string
			if fadeIn > 0 {
				aFilters = append(aFilters, fmt.Sprintf("afade=t=in:st=0:d=%.2f", fadeIn))
			}
			if fadeOut > 0 {
				fadeOutStart := duration - fadeOut
				if fadeOutStart < 0 {
					fadeOutStart = 0
				}
				aFilters = append(aFilters, fmt.Sprintf("afade=t=out:st=%.2f:d=%.2f", fadeOutStart, fadeOut))
			}
			ffmpegArgs = append(ffmpegArgs, "-af", strings.Join(aFilters, ","))
		} else {
			ffmpegArgs = append(ffmpegArgs, "-c:a", "copy")
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_faded"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		ffmpegArgs = append(ffmpegArgs, output)
		_, err = ffmpeg.RunOverwrite(ctx, ffmpegArgs...)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "add_fade"), nil
		}

		result := map[string]interface{}{
			"status":   "ok",
			"output":   output,
			"fade_in":  fadeIn,
			"fade_out": fadeOut,
		}
		return util.SuccessResult(result)
	})
}

func registerAddWatermark(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("add_watermark",
		mcp.WithDescription("Add a text or image watermark overlay to a video."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("text",
			mcp.Description("Text for watermark (use this OR image, not both)"),
		),
		mcp.WithString("image",
			mcp.Description("Path to watermark image (use this OR text, not both)"),
		),
		mcp.WithString("position",
			mcp.Description("Position: top-left, top-right, bottom-left, bottom-right, center (default: bottom-right)"),
		),
		mcp.WithNumber("opacity",
			mcp.Description("Opacity from 0.0 to 1.0 (default: 0.7)"),
		),
		mcp.WithNumber("size",
			mcp.Description("Font size for text watermark (default: 24)"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "add_watermark"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "add_watermark"), nil
		}

		text := util.GetStringArg(args, "text")
		image := util.GetStringArg(args, "image")
		if text == "" && image == "" {
			return util.ErrorResult(util.ErrInvalidInput, "Either text or image is required", "add_watermark"), nil
		}

		position := util.GetStringArg(args, "position")
		if position == "" {
			position = "bottom-right"
		}
		opacity := util.GetFloatArg(args, "opacity", 0.7)
		fontSize := util.GetIntArg(args, "size", 24)

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_watermarked"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		var err error
		if text != "" {
			// Text watermark
			x, y := positionToXY(position)
			filter := fmt.Sprintf("drawtext=text='%s':fontsize=%d:fontcolor=white@%.1f:x=%s:y=%s",
				text, fontSize, opacity, x, y)

			_, err = ffmpeg.RunOverwrite(ctx,
				"-i", input,
				"-vf", filter,
				"-c:a", "copy",
				output,
			)
		} else {
			// Image watermark
			if _, err2 := os.Stat(image); os.IsNotExist(err2) {
				return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("Watermark image not found: %s", image), "add_watermark"), nil
			}
			overlay := positionToOverlay(position)
			filter := fmt.Sprintf("[1:v]format=rgba,colorchannelmixer=aa=%.1f[wm];[0:v][wm]overlay=%s[vout]",
				opacity, overlay)

			_, err = ffmpeg.RunOverwrite(ctx,
				"-i", input,
				"-i", image,
				"-filter_complex", filter,
				"-map", "[vout]",
				"-map", "0:a?",
				"-c:a", "copy",
				output,
			)
		}

		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "add_watermark"), nil
		}

		result := map[string]interface{}{
			"status":   "ok",
			"output":   output,
			"position": position,
		}
		return util.SuccessResult(result)
	})
}

func positionToXY(pos string) (string, string) {
	switch pos {
	case "top-left":
		return "20", "20"
	case "top-right":
		return "w-tw-20", "20"
	case "bottom-left":
		return "20", "h-th-20"
	case "center":
		return "(w-tw)/2", "(h-th)/2"
	default: // bottom-right
		return "w-tw-20", "h-th-20"
	}
}

func positionToOverlay(pos string) string {
	switch pos {
	case "top-left":
		return "20:20"
	case "top-right":
		return "W-w-20:20"
	case "bottom-left":
		return "20:H-h-20"
	case "center":
		return "(W-w)/2:(H-h)/2"
	default: // bottom-right
		return "W-w-20:H-h-20"
	}
}

func registerBlurRegion(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("blur_region",
		mcp.WithDescription("Apply a blur effect to a specific region of the video, optionally for a specific time range."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithNumber("x", mcp.Required(), mcp.Description("X position of blur region (pixels)")),
		mcp.WithNumber("y", mcp.Required(), mcp.Description("Y position of blur region (pixels)")),
		mcp.WithNumber("w", mcp.Required(), mcp.Description("Width of blur region (pixels)")),
		mcp.WithNumber("h", mcp.Required(), mcp.Description("Height of blur region (pixels)")),
		mcp.WithString("start", mcp.Description("Start timestamp for blur (optional, applies to entire video if omitted)")),
		mcp.WithString("end", mcp.Description("End timestamp for blur (optional)")),
		mcp.WithString("output", mcp.Description("Output filename (optional)")),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "blur_region"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "blur_region"), nil
		}

		x := util.GetIntArg(args, "x", 0)
		y := util.GetIntArg(args, "y", 0)
		w := util.GetIntArg(args, "w", 100)
		h := util.GetIntArg(args, "h", 100)

		filter := fmt.Sprintf("[0:v]crop=%d:%d:%d:%d,boxblur=10[blur];[0:v][blur]overlay=%d:%d[vout]",
			w, h, x, y, x, y)

		// If time range specified, add enable
		startTS := util.GetStringArg(args, "start")
		endTS := util.GetStringArg(args, "end")
		if startTS != "" && endTS != "" {
			startSec, err1 := util.TimestampToSeconds(startTS)
			endSec, err2 := util.TimestampToSeconds(endTS)
			if err1 != nil || err2 != nil {
				return util.ErrorResult(util.ErrInvalidTimestamp, "Invalid start or end timestamp", "blur_region"), nil
			}
			filter = fmt.Sprintf("[0:v]crop=%d:%d:%d:%d,boxblur=10[blur];[0:v][blur]overlay=%d:%d:enable='between(t,%.2f,%.2f)'[vout]",
				w, h, x, y, x, y, startSec, endSec)
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_blurred"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-filter_complex", filter,
			"-map", "[vout]",
			"-map", "0:a?",
			"-c:a", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "blur_region"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
			"region": map[string]int{"x": x, "y": y, "w": w, "h": h},
		}
		return util.SuccessResult(result)
	})
}

func registerZoomEffect(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("zoom_effect",
		mcp.WithDescription("Apply a Ken Burns style zoom/pan effect to a video."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithNumber("zoom",
			mcp.Description("Zoom factor: 1.0 = no zoom, 1.5 = 50% zoom in (default: 1.5)"),
		),
		mcp.WithString("direction",
			mcp.Description("Zoom direction: in, out (default: in)"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "zoom_effect"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "zoom_effect"), nil
		}

		zoom := util.GetFloatArg(args, "zoom", 1.5)
		direction := util.GetStringArg(args, "direction")
		if direction == "" {
			direction = "in"
		}

		var filter string
		if direction == "out" {
			filter = fmt.Sprintf("zoompan=z='if(lte(zoom,1.0),%f,max(1.001,zoom-0.0015))':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=1:s=1920x1080:fps=30", zoom)
		} else {
			filter = fmt.Sprintf("zoompan=z='min(zoom+0.0015,%f)':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=1:s=1920x1080:fps=30", zoom)
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_zoomed"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-vf", filter,
			"-c:a", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "zoom_effect"), nil
		}

		result := map[string]interface{}{
			"status":    "ok",
			"output":    output,
			"zoom":      zoom,
			"direction": direction,
		}
		return util.SuccessResult(result)
	})
}

func registerColorGrade(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("color_grade",
		mcp.WithDescription("Apply color grading with built-in presets or manual parameters."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithString("preset",
			mcp.Description("Color preset: warm, cool, cinematic, vintage, none (default: none)"),
		),
		mcp.WithNumber("brightness", mcp.Description("Brightness adjustment: -1.0 to 1.0")),
		mcp.WithNumber("contrast", mcp.Description("Contrast: 0.5 to 2.0 (1.0 = normal)")),
		mcp.WithNumber("saturation", mcp.Description("Saturation: 0.0 to 3.0 (1.0 = normal)")),
		mcp.WithNumber("gamma", mcp.Description("Gamma: 0.5 to 2.0 (1.0 = normal)")),
		mcp.WithString("output", mcp.Description("Output filename (optional)")),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "color_grade"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "color_grade"), nil
		}

		preset := util.GetStringArg(args, "preset")

		var filter string
		switch preset {
		case "warm":
			filter = "eq=saturation=1.2:gamma_r=1.1:gamma_b=0.9"
		case "cool":
			filter = "eq=saturation=1.1:gamma_b=1.15:gamma_r=0.9"
		case "cinematic":
			filter = "eq=contrast=1.1:saturation=0.85,curves=preset=strong_contrast"
		case "vintage":
			filter = "eq=saturation=0.7:gamma=1.05,colorchannelmixer=.393:.769:.189:0:.349:.686:.168:0:.272:.534:.131"
		default:
			// Manual parameters
			brightness := util.GetFloatArg(args, "brightness", 0)
			contrast := util.GetFloatArg(args, "contrast", 1.0)
			saturation := util.GetFloatArg(args, "saturation", 1.0)
			gamma := util.GetFloatArg(args, "gamma", 1.0)
			filter = fmt.Sprintf("eq=brightness=%.2f:contrast=%.2f:saturation=%.2f:gamma=%.2f",
				brightness, contrast, saturation, gamma)
		}

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_graded"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-vf", filter,
			"-c:a", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "color_grade"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
			"preset": preset,
		}
		return util.SuccessResult(result)
	})
}

func registerSpeedChange(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("speed_change",
		mcp.WithDescription("Change video playback speed. Audio is adjusted to stay in sync."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to input video file"),
		),
		mcp.WithNumber("speed",
			mcp.Required(),
			mcp.Description("Speed multiplier: 0.25 = quarter speed, 2.0 = double speed, 4.0 = 4x"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "speed_change"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "speed_change"), nil
		}

		speed := util.GetFloatArg(args, "speed", 1.0)
		if speed <= 0 || speed > 100 {
			return util.ErrorResult(util.ErrInvalidInput, "speed must be between 0.01 and 100", "speed_change"), nil
		}

		pts := 1.0 / speed
		// atempo only supports 0.5-2.0, chain multiple for larger ranges
		atempoFilters := buildAtempoChain(speed)

		filter := fmt.Sprintf("[0:v]setpts=%.4f*PTS[v];[0:a]%s[a]", pts, atempoFilters)

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, fmt.Sprintf("%s_%gx%s", base, speed, ext))
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		// Try with audio first; if input has no audio, retry video-only
		_, err := ffmpeg.RunOverwrite(ctx,
			"-i", input,
			"-filter_complex", filter,
			"-map", "[v]",
			"-map", "[a]",
			output,
		)
		if err != nil {
			// Fallback: video has no audio track, process video only
			videoOnlyFilter := fmt.Sprintf("[0:v]setpts=%.4f*PTS[v]", pts)
			_, err = ffmpeg.RunOverwrite(ctx,
				"-i", input,
				"-filter_complex", videoOnlyFilter,
				"-map", "[v]",
				output,
			)
		}
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "speed_change"), nil
		}

		result := map[string]interface{}{
			"status": "ok",
			"output": output,
			"speed":  speed,
		}
		return util.SuccessResult(result)
	})
}

// buildAtempoChain chains atempo filters since each only supports 0.5-2.0 range.
func buildAtempoChain(speed float64) string {
	if speed >= 0.5 && speed <= 2.0 {
		return fmt.Sprintf("atempo=%.4f", speed)
	}

	var parts []string
	remaining := speed
	for remaining > 2.0 {
		parts = append(parts, "atempo=2.0")
		remaining /= 2.0
	}
	for remaining < 0.5 {
		parts = append(parts, "atempo=0.5")
		remaining /= 0.5
	}
	parts = append(parts, fmt.Sprintf("atempo=%.4f", remaining))
	return strings.Join(parts, ",")
}

func registerAddIntroOutro(s *server.MCPServer, cfg *config.Config, ffmpeg *util.FfmpegRunner) {
	tool := mcp.NewTool("add_intro_outro",
		mcp.WithDescription("Prepend an intro and/or append an outro to a video."),
		mcp.WithString("input",
			mcp.Required(),
			mcp.Description("Path to main video file"),
		),
		mcp.WithString("intro",
			mcp.Description("Path to intro video file (prepended)"),
		),
		mcp.WithString("outro",
			mcp.Description("Path to outro video file (appended)"),
		),
		mcp.WithString("output",
			mcp.Description("Output filename (optional)"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		input := util.GetStringArg(args, "input")
		if input == "" {
			return util.ErrorResult(util.ErrInvalidInput, "input is required", "add_intro_outro"), nil
		}
		if _, err := os.Stat(input); os.IsNotExist(err) {
			return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("File not found: %s", input), "add_intro_outro"), nil
		}

		intro := util.GetStringArg(args, "intro")
		outro := util.GetStringArg(args, "outro")
		if intro == "" && outro == "" {
			return util.ErrorResult(util.ErrInvalidInput, "At least one of intro or outro is required", "add_intro_outro"), nil
		}

		// Build file list
		var files []string
		if intro != "" {
			if _, err := os.Stat(intro); os.IsNotExist(err) {
				return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("Intro file not found: %s", intro), "add_intro_outro"), nil
			}
			files = append(files, intro)
		}
		files = append(files, input)
		if outro != "" {
			if _, err := os.Stat(outro); os.IsNotExist(err) {
				return util.ErrorResult(util.ErrFileNotFound, fmt.Sprintf("Outro file not found: %s", outro), "add_intro_outro"), nil
			}
			files = append(files, outro)
		}

		concatFile := filepath.Join(cfg.TempDir, "intro_outro_list.txt")
		var content strings.Builder
		for _, f := range files {
			content.WriteString(fmt.Sprintf("file '%s'\n", f))
		}
		if err := os.WriteFile(concatFile, []byte(content.String()), 0o644); err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "add_intro_outro"), nil
		}
		defer os.Remove(concatFile)

		output := util.GetStringArg(args, "output")
		if output == "" {
			ext := filepath.Ext(input)
			base := strings.TrimSuffix(filepath.Base(input), ext)
			output = filepath.Join(cfg.OutputDir, base+"_final"+ext)
		} else if !filepath.IsAbs(output) {
			output = filepath.Join(cfg.OutputDir, output)
		}

		_, err := ffmpeg.RunOverwrite(ctx,
			"-f", "concat",
			"-safe", "0",
			"-i", concatFile,
			"-c", "copy",
			output,
		)
		if err != nil {
			return util.ErrorResult(util.ErrFfmpegEncodeError, err.Error(), "add_intro_outro"), nil
		}

		result := map[string]interface{}{
			"status":    "ok",
			"output":    output,
			"has_intro": intro != "",
			"has_outro": outro != "",
		}
		return util.SuccessResult(result)
	})
}
