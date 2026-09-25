#!/usr/bin/env bash
# Re-records demo.gif from demo.tape. vhs only captures the frames; ffmpeg
# builds the GIF directly, because vhs 0.12 fails silently with ffmpeg 9.
# Requires vhs, ttyd, ffmpeg. Usage, from the repo root: scripts/record-demo.sh
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
sed "s#^Output .*#Output \"$tmp/frames/\"#" demo.tape > "$tmp/demo.tape"
vhs "$tmp/demo.tape"
speed=$(sed -n 's/^Set PlaybackSpeed //p' demo.tape); speed=${speed:-1}
ffmpeg -y -loglevel error \
  -framerate 50 -i "$tmp/frames/frame-text-%05d.png" \
  -framerate 50 -i "$tmp/frames/frame-cursor-%05d.png" \
  -filter_complex "[0][1]overlay,setpts=PTS/${speed},fps=20,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" \
  demo.gif
echo "demo.gif: $(du -h demo.gif | cut -f1)"
