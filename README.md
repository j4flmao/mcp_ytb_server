# mcp_ytb_server (Video Pipeline MCP)

A Go-based MCP (stdio) server that provides a video processing pipeline: download (yt-dlp), trim/merge/encode (ffmpeg), subtitles (yt-dlp / whisper), effects, and export.

Repo: https://github.com/j4flmao/mcp_ytb_server

## Requirements

- `yt-dlp` (required)
- `ffmpeg` + `ffprobe` (required)
- `whisper` / `whisper.cpp` (optional; only needed if you use Whisper subtitle generation tools)

## Installation

### Option 1: Run via `npx` (recommended; no Go required)

This repo includes an npm wrapper in [npm/](file:///d:/mcp_ytb_server/npm) so end users only need Node.js + `npx`.

Claude Desktop config (Windows example):

- Config file path: `%APPDATA%\\Claude\\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mcp-ytb-server": {
      "command": "npx",
      "args": ["-y", "@mcp_ytb/mcp_ytb_server"]
    }
  }
}
```

Optional configuration (manual):

- In Claude Desktop, you can add an `env` object under `mcp-ytb-server` to set defaults (for example output dir, default quality, cookies).
- In prompts, you can also override per call (for example “save to D:\Videos” or “use cookies file …”).

For `npx` publishing, place binaries at:

- `npm/bin/windows-amd64/video-mcp.exe`
- `npm/bin/linux-amd64/video-mcp`
- `npm/bin/darwin-amd64/video-mcp`
- `npm/bin/darwin-arm64/video-mcp`

### Option 2: Use a prebuilt binary (advanced)

- Download the correct binary for your OS from GitHub Releases and place it in a stable path.

### Option 3: Build from source (advanced; requires Go)

```powershell
cd D:\mcp_ytb_server
go test ./...
go build -o .\video-mcp.exe .
```

## Claude Desktop Configuration

Config file (Windows):

- `%APPDATA%\\Claude\\claude_desktop_config.json`

Example config using the `.exe` binary (optional):

```json
{
  "mcpServers": {
    "mcp-ytb-server": {
      "command": "D:\\\\mcp_ytb_server\\\\video-mcp.exe",
      "args": []
    }
  }
}
```

Restart Claude Desktop (fully quit and reopen) after updating the config.

## Environment Variables

- `VIDEO_MCP_OUTPUT_DIR`: default output directory (optional)
- `VIDEO_MCP_TEMP_DIR`: temp directory (optional)
- `VIDEO_MCP_YTDLP_PATH`: path to `yt-dlp` (default: `yt-dlp`)
- `VIDEO_MCP_FFMPEG_PATH`: path to `ffmpeg` (default: `ffmpeg`)
- `VIDEO_MCP_FFPROBE_PATH`: path to `ffprobe` (default: `ffprobe`)
- `VIDEO_MCP_QUALITY`: default download quality (default: `best`)
- `VIDEO_MCP_COOKIE_BROWSER`: browser for yt-dlp browser-cookie extraction (e.g. `chrome`, `firefox`, `none`)
- `VIDEO_MCP_COOKIE_FILE`: path to `cookies.txt` (Netscape format); takes priority over browser cookies

## Cookies (YouTube login / age-gate)

There are two options:

- Set `VIDEO_MCP_COOKIE_FILE` so all requests use `cookies.txt` by default (optional).
- In your prompt, explicitly specify the cookie file path; the `download_video` tool supports `cookie_file` (optional, per call).

## Prompt Samples

See [PROMPTS.md](file:///d:/mcp_ytb_server/PROMPTS.md) for copy/paste prompts to test.

## Main Tools

- `download_video`: download video/audio from a URL (yt-dlp)
- `get_video_info`: fetch metadata from a URL
- `download_subtitles`, `list_subtitles`: subtitles via yt-dlp
- `trim_video`, `cut_and_keep`, `merge_clips`, `split_video`
- `mute_audio`, `replace_audio`, `normalize_audio`
- `add_fade`, `add_watermark`, `blur_region`, `zoom_effect`, `color_grade`
- `export_video`, `generate_thumbnail`, `get_file_info`

## Quick Troubleshooting

- Windows Media Player can't play the file: you may be hitting codec issues (AV1/VP9/HEVC). Use `export_video` to re-encode to MP4 H.264/AAC.
- Login/age-gated video: configure cookies (see Cookies section).
- Claude doesn't see updated tools: fully quit Claude Desktop (system tray) and reopen.
