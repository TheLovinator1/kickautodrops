# KickAutoDrops

This repository is a fork of the original [KickAutoDrops](https://github.com/PBA4EVSKY/kickautodrops), LLM was used to convert the Python codebase to Go, and add a TUI interface for easier use.

KickAutoDrops is a minimalist automation tool designed to efficiently collect Rust game drops from Kick.com without actually streaming any video or audio content. The application runs in the background, simulating stream viewing by interacting with Kick.com's API, allowing you to collect drops while saving bandwidth and system resources.

## Installation

### Option 1: Pre-built Binary

1. Download the latest binary for your platform from the [Releases](https://github.com/TheLovinator1/kickautodrops/releases) page
2. Extract the executable
3. Install a cookie export extension for your browser:
   - [Chrome](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
   - [Firefox](https://addons.mozilla.org/en-US/firefox/addon/get-cookies-txt-locally/)
4. Go to [kick.com](https://kick.com), log in, and export all cookies using the extension
5. Place the exported `cookies.txt` file next to the executable
6. Run from terminal/command prompt

### Option 2: Build from Source

**Prerequisites:** [Go 1.26+](https://go.dev/dl/)

```bash
# Clone the repository
git clone https://github.com/TheLovinator1/kickautodrops.git

# Navigate to the directory
cd kickautodrops

# Build the binary
go build -o kickautodrops .

# Run it
./kickautodrops
```
