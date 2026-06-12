# KickAutoDrops

![Screenshot of the KickAutoDrops TUI application. The screen is split into two panels: a scrolling log on the left and a reward tree on the right. Log entries show the application pinging, handshaking with channel 74469958, making API GET requests to kick.com, and updating watch progress for streamer spoonkid (from 46.0 to 45.0 minutes remaining, a decrease of 1.0 minute watched). It also reports spoonkid is LIVE playing game 13, and that rewards like Kick + Rust Wallpaper Pattern have progress 1.0 and are already claimed. The right-side reward tree lists campaign rewards with their claim status: Kick + Rust Wallpaper Pack (no status shown), Kick + Rust Wallpaper Logo [Claimed], Kick + Rust Wallpaper Pattern [Claimed], Team CoconutB + Winnie Backpack [Claimed], Team Erobb + Angelaoreo M249 [0%], Team Oilrats + Trausi Gloves [Claimed], Team Spoonkid + Throat Pickaxe [62%], Team Willjum + Sinks Crossbow [Claimed], and Team hJune + Frost AR [0%]. A footer bar shows the time 13:53:08 over 10:46:00, the streamer name spoonkid, the status Watching..., and the hint Ctrl+S save logs to disk. The overall tone is functional and technical, displaying the automated drop collection process in real time.](.github/workflows/20260612_160903.webp)

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
