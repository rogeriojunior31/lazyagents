#!/usr/bin/env bash
# Records demos/<name>.tape into docs/assets/demos/<name>.{gif,mp4,webp}:
#   gif  — framed window (title bar, rounded corners, shadow) for the README and guides
#   mp4  — the bare terminal at full 2x resolution, for the website (it draws its own frame)
#   webp — poster for the mp4 (last frame)
# vhs only captures the frames; ffmpeg encodes, because vhs 0.12 fails silently
# with ffmpeg 9. Requires vhs, ttyd, ffmpeg, magick.
# Usage, from the repo root: scripts/record-demo.sh [name...]   (default: every tape)
set -euo pipefail
# memory cap: a runaway vhs/ffmpeg is killed alone, not the whole terminal
if [[ -z ${RECORD_DEMO_CAPPED:-} ]] && command -v systemd-run >/dev/null; then
  RECORD_DEMO_CAPPED=1 exec systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=0 "$0" "$@"
fi
cd "$(dirname "$0")/.."
out=docs/assets/demos
mkdir -p "$out"

# chrome, in 2x pixels; the gif is then scaled down by GIF_SCALE
margin=88 bar=60 radius=22 GIF_SCALE=${GIF_SCALE:-0.75}
bg_from='#232640' bg_to='#0f101a' win_bar='#0f101a' term_bg='#151723' title_fg='#707380'

record() {
  local name=$1 tape=demos/$1.tape tmp
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN
  { echo "Output \"$tmp/frames/\""; grep -v '^Output ' "$tape"; } > "$tmp/run.tape"
  vhs -q "$tmp/run.tape"

  local speed w h W H title
  speed=$(sed -n 's/^Set PlaybackSpeed //p' "$tape"); speed=${speed:-1}
  title=$(sed -n 's/^# title: //p' "$tape"); title=${title:-lazyagents}
  read -r w h < <(magick identify -format '%w %h\n' "$tmp/frames/frame-text-00001.png")
  W=$((w + 2 * margin)) H=$((h + bar + 2 * margin))

  # bg.png: gradient + shadow + window (bar with traffic lights and title);
  # cut.png: the same backdrop with the rounded window punched out, laid over
  # the terminal so its square corners disappear
  magick -size "${W}x${H}" "gradient:${bg_from}-${bg_to}" \
    \( -size "${W}x${H}" xc:none -fill 'rgba(0,0,0,0.55)' \
       -draw "roundrectangle $margin,$((margin + 18)) $((W - margin)),$((H - margin + 18)) $radius,$radius" \
       -blur 0x28 \) -composite "$tmp/backdrop.png"
  magick "$tmp/backdrop.png" \
    -fill "$term_bg" -draw "roundrectangle $margin,$margin $((W - margin - 1)),$((H - margin - 1)) $radius,$radius" \
    -fill "$win_bar" -draw "roundrectangle $margin,$margin $((W - margin - 1)),$((margin + bar + radius)) $radius,$radius" \
    -fill "$term_bg" -draw "rectangle $margin,$((margin + bar)) $((W - margin - 1)),$((margin + bar + radius))" \
    -fill '#ff5f57' -draw "circle $((margin + 34)),$((margin + bar / 2)) $((margin + 34 + 11)),$((margin + bar / 2))" \
    -fill '#febc2e' -draw "circle $((margin + 70)),$((margin + bar / 2)) $((margin + 70 + 11)),$((margin + bar / 2))" \
    -fill '#28c840' -draw "circle $((margin + 106)),$((margin + bar / 2)) $((margin + 106 + 11)),$((margin + bar / 2))" \
    -font "$(fc-match -f '%{file}' 'MesloLGM Nerd Font Mono')" -pointsize 24 -fill "$title_fg" -gravity north \
    -annotate "+0+$((margin + bar / 2 - 15))" "$title" "$tmp/bg.png"
  magick "$tmp/backdrop.png" \
    \( -size "${W}x${H}" xc:white -fill black \
       -draw "roundrectangle $margin,$margin $((W - margin - 1)),$((H - margin - 1)) $radius,$radius" \) \
    -alpha off -compose copy_opacity -composite "$tmp/cut.png"

  local fps
  fps=$(sed -n 's/^Set Framerate //p' demos/setup.tape "$tape" | tail -1); fps=${fps:-50}
  local frames=(-framerate "$fps" -i "$tmp/frames/frame-text-%05d.png" -framerate "$fps" -i "$tmp/frames/frame-cursor-%05d.png")
  local term="[0][1]overlay,setpts=PTS/${speed}"

  # every overlay ends with the frames (shortest=1): a looped png is an endless
  # input, and an endless stream into palettegen buffers until the OOM killer.
  # Two passes keep memory flat: palette to a file, then paletteuse.
  local framed="${term}[t];[2][t]overlay=${margin}:$((margin + bar)):shortest=1[c];[c][3]overlay=shortest=1,scale=iw*${GIF_SCALE}:-2:flags=lanczos,fps=25"
  local inputs=("${frames[@]}" -loop 1 -i "$tmp/bg.png" -loop 1 -i "$tmp/cut.png")
  ffmpeg -y -loglevel error "${inputs[@]}" -filter_complex "${framed},palettegen=max_colors=256:stats_mode=full" -frames:v 1 "$tmp/palette.png"
  ffmpeg -y -loglevel error "${inputs[@]}" -i "$tmp/palette.png" \
    -filter_complex "${framed}[g];[g][4]paletteuse=dither=sierra2_4a:diff_mode=rectangle" "$out/$name.gif"

  ffmpeg -y -loglevel error "${frames[@]}" -filter_complex "${term},fps=30,crop=trunc(iw/2)*2:trunc(ih/2)*2" \
    -c:v libx264 -preset slow -crf 18 -tune animation -pix_fmt yuv420p -movflags +faststart "$out/$name.mp4"

  local last
  last=$(find "$tmp/frames" -name 'frame-text-*' | sort | tail -1)
  magick "$last" "${last/text/cursor}" -composite -quality 90 "$out/$name.webp"

  du -h "$out/$name".{gif,mp4,webp} | sed 's/^/  /'
}

if (($#)); then names=("$@"); else
  names=(); for t in demos/*.tape; do [[ $t == */setup.tape ]] || names+=("$(basename "$t" .tape)"); done
fi
for n in "${names[@]}"; do echo "$n"; record "$n"; done
