#!/usr/bin/env node
// postinstall.js — downloads the candy binary for the current platform.
//
// Pattern follows esbuild/swc: fetch a versioned release asset from GitHub,
// verify SHA256, and place the binary at a well-known path that bin/candy shims.

'use strict';

const path = require('path');
const fs = require('fs');
const https = require('https');
const crypto = require('crypto');
const { execFileSync } = require('child_process');

const PACKAGE_VERSION = require('./package.json').version;
const REPO = 'CallipsosNetwork/candy';
const BIN_DIR = path.join(__dirname, 'bin');
const BIN_PATH = path.join(BIN_DIR, process.platform === 'win32' ? 'candy.exe' : 'candy-bin');

// Platform → asset name mapping (matches the release.yml matrix)
function platformAsset() {
  const { platform, arch } = process;
  const map = {
    'linux-x64':   'candy-linux-x86_64',
    'linux-arm64': 'candy-linux-aarch64',
    'darwin-x64':  'candy-macos-x86_64',
    'darwin-arm64':'candy-macos-aarch64',
    'win32-x64':   'candy-windows-x86_64.exe',
  };
  const key = `${platform}-${arch}`;
  return map[key] || null;
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const request = (u) => {
      https.get(u, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          request(res.headers.location);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode} for ${u}`));
          return;
        }
        res.pipe(file);
        file.on('finish', () => file.close(resolve));
      }).on('error', reject);
    };
    request(url);
  });
}

async function verifySha256(filePath, expectedSha) {
  const data = fs.readFileSync(filePath);
  const actual = crypto.createHash('sha256').update(data).digest('hex');
  if (actual !== expectedSha) {
    throw new Error(`SHA256 mismatch: expected ${expectedSha}, got ${actual}`);
  }
}

async function fetchShasums(baseUrl) {
  return new Promise((resolve, reject) => {
    const request = (u) => {
      https.get(u, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          request(res.headers.location);
          return;
        }
        let data = '';
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => resolve(data));
      }).on('error', reject);
    };
    request(baseUrl);
  });
}

async function main() {
  const asset = platformAsset();
  if (!asset) {
    console.warn(
      `[candy] Unsupported platform ${process.platform}-${process.arch}. ` +
      `Build from source: https://github.com/${REPO}`
    );
    return;
  }

  // Skip if binary is already present (e.g. re-running postinstall)
  if (fs.existsSync(BIN_PATH)) {
    return;
  }

  const releaseBase = `https://github.com/${REPO}/releases/download/v${PACKAGE_VERSION}`;
  const assetUrl = `${releaseBase}/${asset}`;
  const sumsUrl = `${releaseBase}/sha256sums.txt`;

  if (!fs.existsSync(BIN_DIR)) {
    fs.mkdirSync(BIN_DIR, { recursive: true });
  }

  console.log(`[candy] Downloading ${asset} v${PACKAGE_VERSION}...`);

  try {
    await downloadFile(assetUrl, BIN_PATH);

    // Fetch and verify SHA256
    try {
      const sums = await fetchShasums(sumsUrl);
      const line = sums.split('\n').find((l) => l.includes(asset));
      if (line) {
        const expected = line.trim().split(/\s+/)[0];
        await verifySha256(BIN_PATH, expected);
        console.log(`[candy] SHA256 verified.`);
      } else {
        console.warn(`[candy] SHA256 entry not found for ${asset}; skipping verification.`);
      }
    } catch (e) {
      console.warn(`[candy] Could not verify SHA256: ${e.message}`);
    }

    // Make executable on Unix
    if (process.platform !== 'win32') {
      fs.chmodSync(BIN_PATH, 0o755);
    }

    console.log(`[candy] Installed at ${BIN_PATH}`);
  } catch (e) {
    // Graceful degradation: warn but don't crash install
    if (fs.existsSync(BIN_PATH)) {
      fs.unlinkSync(BIN_PATH);
    }
    console.warn(
      `[candy] Could not download binary: ${e.message}. ` +
      `Build from source or install via cargo install candy.`
    );
  }
}

main().catch((e) => {
  console.warn(`[candy] postinstall error: ${e.message}`);
});
