const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");
const { chmodSync } = require("node:fs");

function download(url, destPath) {
  return new Promise((resolve, reject) => {
    const request = https.get(
      url,
      {
        headers: {
          "User-Agent": "mcp-ytb-server-npm-wrapper"
        }
      },
      (res) => {
        if (
          res.statusCode &&
          res.statusCode >= 300 &&
          res.statusCode < 400 &&
          res.headers.location
        ) {
          res.resume();
          resolve(download(res.headers.location, destPath));
          return;
        }

        if (res.statusCode !== 200) {
          res.resume();
          reject(new Error(`Download failed: ${res.statusCode} ${res.statusMessage || ""}`));
          return;
        }

        const tmpPath = destPath + ".tmp";
        fs.mkdirSync(path.dirname(destPath), { recursive: true });
        const file = fs.createWriteStream(tmpPath);
        res.pipe(file);
        file.on("finish", () => {
          file.close(() => {
            fs.renameSync(tmpPath, destPath);
            resolve();
          });
        });
        file.on("error", (err) => {
          try {
            file.close(() => {});
          } catch {}
          reject(err);
        });
      }
    );
    request.on("error", reject);
  });
}

function resolveAssetName() {
  const platform = process.platform;
  const arch = process.arch;

  if (platform === "win32") {
    if (arch !== "x64") throw new Error(`Unsupported arch on Windows: ${arch}`);
    return "video-mcp-windows-amd64.exe";
  }
  if (platform === "darwin") {
    if (arch === "arm64") return "video-mcp-darwin-arm64";
    if (arch === "x64") return "video-mcp-darwin-amd64";
    throw new Error(`Unsupported arch on macOS: ${arch}`);
  }
  if (platform === "linux") {
    if (arch === "arm64") return "video-mcp-linux-arm64";
    if (arch === "x64") return "video-mcp-linux-amd64";
    throw new Error(`Unsupported arch on Linux: ${arch}`);
  }
  throw new Error(`Unsupported platform: ${platform}`);
}

async function main() {
  const version = process.env.npm_package_version || "0.0.0";
  const asset = resolveAssetName();
  const root = path.resolve(__dirname, "..");
  const nativeDir = path.join(__dirname, "native");
  const binPath = path.join(nativeDir, asset);

  if (fs.existsSync(binPath)) return;

  const tag = `v${version}`;
  const url = `https://github.com/j4flmao/mcp_ytb_server/releases/download/${tag}/${asset}`;

  try {
    await download(url, binPath);
    if (process.platform !== "win32") {
      chmodSync(binPath, 0o755);
    }
  } catch (err) {
    const msg = [
      "",
      "Failed to download mcp_ytb_server binary.",
      `Tried: ${url}`,
      "",
      "Fix options:",
      "- Ensure the GitHub release exists and contains the expected assets.",
      "- Or install from source with Go and point Claude Desktop to the built binary.",
      ""
    ].join(os.EOL);
    process.stderr.write(msg);
    throw err;
  }
}

main().catch((err) => {
  process.stderr.write(String(err) + os.EOL);
  process.exit(1);
});

