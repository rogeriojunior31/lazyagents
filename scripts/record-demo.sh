#!/usr/bin/env bash
# Records demos/<name>.tape into docs/assets/demos/<name>.{gif,mp4,webp}:
#   gif  — framed window (title bar, rounded corners, shadow) for the README and guides
#   mp4  — the bare terminal at full capture resolution, for the website (it draws its own frame)
#   webp — poster for the mp4 (last frame)
# vhs only captures the frames; ffmpeg encodes, because vhs 0.12 fails silently
# with ffmpeg 9. Requires Go, vhs, ttyd, ffmpeg, magick, sqlite3 and Python 3.
# Usage, from the repo root: scripts/record-demo.sh [name...]   (default: every tape)
set -euo pipefail
# memory cap: a runaway vhs/ffmpeg is killed alone, not the whole terminal
if [[ -z ${RECORD_DEMO_CAPPED:-} ]] && command -v systemd-run >/dev/null; then
  RECORD_DEMO_CAPPED=1 exec systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=0 "$0" "$@"
fi
cd "$(dirname "$0")/.."
out=docs/assets/demos
mkdir -p "$out"

# chrome, in capture pixels; the gif is then scaled down by GIF_SCALE
margin=64 bar=56 radius=20 GIF_SCALE=${GIF_SCALE:-0.60}
bg_from='#232640' bg_to='#0f101a' win_bar='#0f101a' term_bg='#151723' title_fg='#707380'

VHS_BIN=${VHS_BIN:-vhs}
for tool in go ttyd ffmpeg ffprobe magick sqlite3 python3 fc-match "$VHS_BIN"; do
  command -v "$tool" >/dev/null || { echo "missing demo tool: $tool" >&2; exit 1; }
done
[[ $(fc-match -f '%{family}' 'MesloLGM Nerd Font Mono') == *'MesloLGM Nerd Font Mono'* ]] || {
  echo 'missing demo font: MesloLGM Nerd Font Mono' >&2; exit 1;
}
"$VHS_BIN" --version >/dev/null || { echo 'vhs failed; set VHS_BIN to its installed executable' >&2; exit 1; }
work=$(mktemp -d /tmp/lazyagents-record.XXXXXX)
cleanup() { python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1])' "$work"; }
trap cleanup EXIT
go build -ldflags '-X main.version=demo' -o "$work/lazyagents" .

record() {
  local name=$1 tape=demos/$1.tape tmp
  tmp="$work/$name"
  mkdir -p "$tmp"
  if ! scripts/demo-home.sh "$tmp/home" "$work/lazyagents" >"$tmp/seed.log" 2>&1; then
    cat "$tmp/seed.log" >&2
    return 1
  fi
  { echo "Output \"$tmp/frames/\""; grep -v '^Output ' "$tape"; } > "$tmp/run.tape"
  env -i HOME="$tmp/home" PATH="$(dirname "$(command -v ttyd)"):/usr/bin:/bin" \
    TERM=xterm-256color LANG=C.UTF-8 TZ=UTC \
    LAZYAGENTS_DEMO_BINARY="$work/lazyagents" "$VHS_BIN" -q "$tmp/run.tape"

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
  local frames=(-threads 2 -framerate "$fps" -i "$tmp/frames/frame-text-%05d.png" -threads 2 -framerate "$fps" -i "$tmp/frames/frame-cursor-%05d.png")
  local term="[0][1]overlay,setpts=PTS/${speed}"

  # every overlay ends with the frames (shortest=1): a looped png is an endless
  # input, and an endless stream into palettegen buffers until the OOM killer.
  # Two passes keep memory flat: palette to a file, then paletteuse.
  local framed="${term}[t];[2][t]overlay=${margin}:$((margin + bar)):shortest=1[c];[c][3]overlay=shortest=1,scale=iw*${GIF_SCALE}:-2:flags=lanczos,fps=20"
  local inputs=("${frames[@]}" -threads 2 -loop 1 -i "$tmp/bg.png" -threads 2 -loop 1 -i "$tmp/cut.png")
  ffmpeg -y -loglevel error -filter_complex_threads 2 -threads 2 "${inputs[@]}" -filter_complex "${framed},palettegen=max_colors=256:stats_mode=full" -frames:v 1 "$tmp/palette.png"
  ffmpeg -y -loglevel error -filter_complex_threads 2 -threads 2 "${inputs[@]}" -threads 2 -i "$tmp/palette.png" \
    -filter_complex "${framed}[g];[g][4]paletteuse=dither=sierra2_4a:diff_mode=rectangle" "$tmp/$name.gif"

  ffmpeg -y -loglevel error -filter_complex_threads 2 -threads 2 "${frames[@]}" -filter_complex "${term},fps=30,crop=trunc(iw/2)*2:trunc(ih/2)*2" \
    -c:v libx264 -preset slow -crf 18 -tune animation -pix_fmt yuv420p -threads 2 -movflags +faststart "$tmp/$name.mp4"

  local last
  last=$(find "$tmp/frames" -name 'frame-text-*' | sort | tail -1)
  magick "$last" "${last/text/cursor}" -composite -quality 90 "$tmp/$name.webp"

  ffprobe -v error -show_entries stream=width,height -of csv=p=0 "$tmp/$name.mp4"
  for ext in gif mp4 webp; do mv "$tmp/$name.$ext" "$out/$name.$ext"; done
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1])' "$tmp"
  du -h "$out/$name".{gif,mp4,webp} | sed 's/^/  /'
}

if (($#)); then names=("$@"); else
  names=(); for t in demos/*.tape; do [[ $t == */setup.tape ]] || names+=("$(basename "$t" .tape)"); done
fi
for n in "${names[@]}"; do
  [[ "$n" =~ ^[a-z]+$ && "$n" != setup && -f "demos/$n.tape" ]] || { echo "unknown demo: $n" >&2; exit 1; }
done
for n in "${names[@]}"; do echo "$n"; record "$n"; done
