#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
FPS=30
DURATION_MS=13000
HOLD_MS=1500                 # freeze the final resolved frame so it doesn't cut abruptly
FRAMES=$(( FPS * DURATION_MS / 1000 ))
HOLD_FRAMES=$(( FPS * HOLD_MS / 1000 ))
FRAMEDIR="frames"
SCENE="file://$(pwd)/scene.html"

rm -rf "$FRAMEDIR"; mkdir -p "$FRAMEDIR"
echo "Rendering $FRAMES frames at ${FPS}fps (1920x1080) + ${HOLD_FRAMES} hold frames…"

for ((i=0; i<FRAMES; i++)); do
  t=$(( i * 1000 / FPS ))
  printf -v out "$FRAMEDIR/f_%05d.png" "$i"
  "$CHROME" --headless --disable-gpu --hide-scrollbars \
    --window-size=1920,1080 --force-device-scale-factor=1 \
    --default-background-color=ff000000 \
    --virtual-time-budget=400 \
    --screenshot="$out" "${SCENE}?t=${t}" >/dev/null 2>&1
  if (( i % 30 == 0 )); then echo "  frame $i / $FRAMES"; fi
done

# Hold: duplicate the last rendered frame so the brand card lingers ~1.5s.
LAST=$(printf "$FRAMEDIR/f_%05d.png" $(( FRAMES - 1 )))
for ((h=0; h<HOLD_FRAMES; h++)); do
  printf -v out "$FRAMEDIR/f_%05d.png" $(( FRAMES + h ))
  cp "$LAST" "$out"
done

echo "Stitching with ffmpeg…"
ffmpeg -y -framerate $FPS -i "$FRAMEDIR/f_%05d.png" \
  -c:v libx264 -pix_fmt yuv420p -crf 18 -preset slow \
  -movflags +faststart \
  burnt-launch.mp4 >/dev/null 2>&1

echo "Done → $(pwd)/burnt-launch.mp4"
ls -lh burnt-launch.mp4 | awk '{print "size:", $5}'
