#!/usr/bin/env bash
# Nested i3/Xvfb: never touches the user's active compositor or configuration.
set -euo pipefail
root=$(cd -- "$(dirname -- "$0")/.." && pwd)
python=${CLIPDECK_TEST_PYTHON:-python3}
for tool in Xvfb i3 i3-msg xdotool; do command -v "$tool" >/dev/null || { echo "Missing test dependency: $tool" >&2; exit 1; }; done
tmp=$(mktemp -d)
trap 'kill "${ui:-}" "${wm:-}" "${server:-}" 2>/dev/null || true; rm -rf "$tmp"' EXIT
Xvfb -displayfd 3 -screen 0 1280x900x24 -nolisten tcp 3>"$tmp/display" >"$tmp/xvfb.log" 2>&1 &
server=$!
for ((i=0;i<100;i++)); do [[ -s "$tmp/display" ]] && break; sleep .05; done
export DISPLAY=":$(cat "$tmp/display")" XDG_SESSION_TYPE=x11 XDG_RUNTIME_DIR="$tmp/runtime"
unset WAYLAND_DISPLAY I3SOCK SWAYSOCK
mkdir -m700 "$XDG_RUNTIME_DIR"
printf '%s\n' 'font pango:monospace 10' 'for_window [class="clipdeck-ui"] floating enable, move position center' >"$tmp/i3.config"
i3 -c "$tmp/i3.config" >"$tmp/i3.log" 2>&1 &
wm=$!
for ((i=0;i<100;i++)); do i3-msg -t get_tree >/dev/null 2>&1 && break; sleep .05; done
export PYTHONPATH="$root/desktop" QT_QPA_PLATFORM=xcb QT_QUICK_BACKEND=software
"$python" -m clipdeck_ui >"$tmp/ui.log" 2>&1 &
ui=$!
for ((i=0;i<100;i++)); do xdotool search --class clipdeck-ui >/dev/null 2>&1 && break; kill -0 "$ui" 2>/dev/null || { cat "$tmp/ui.log" >&2; exit 1; }; sleep .1; done
# WM_CLASS appears before i3 necessarily applies its manage-time rules.
for ((i=0;i<100;i++)); do
  i3-msg -t get_tree >"$tmp/tree.json"
  if "$python" - "$tmp/tree.json" <<'PY'
import json, sys
def ready(node):
    if node.get('window_properties', {}).get('class') == 'clipdeck-ui':
        return node.get('floating') in ('user_on', 'auto_on')
    return any(ready(child) for child in node.get('nodes', []) + node.get('floating_nodes', []))
sys.exit(0 if ready(json.load(open(sys.argv[1]))) else 1)
PY
  then break; fi
  sleep .05
done
"$python" - "$tmp/tree.json" <<'PY'
import json, sys
def find(node):
    if node.get('window_properties', {}).get('class', '').lower() == 'clipdeck-ui':
        return node
    for child in node.get('nodes', []) + node.get('floating_nodes', []):
        result = find(child)
        if result: return result
node = find(json.load(open(sys.argv[1])))
assert node, 'ClipDeck window missing from i3 tree'
assert node.get('floating') in ('user_on', 'auto_on'), node
print('PASS: standalone popup runs as a floating i3/X11 window')
PY
xdotool key Escape
"$python" -m clipdeck_ui
kill -0 "$ui"
echo 'PASS: second invocation activates the existing popup'
"$python" - "$root" <<'PY'
import os, subprocess, sys, time
from PySide6.QtWidgets import QApplication, QTextEdit
app = QApplication([])
editor = QTextEdit()
editor.setWindowTitle('ClipDeck paste acceptance target')
editor.show()
editor.setFocus()
app.processEvents()
target = str(int(editor.winId()))
# Put the popup in front: Paste must restore the original target itself.
popup = subprocess.check_output(['xdotool', 'search', '--class', 'clipdeck-ui'], text=True).splitlines()[0]
subprocess.run(['xdotool', 'windowactivate', '--sync', popup], check=True)
env = dict(os.environ, CLIPDECK_TEST_PASTE_TARGET=target)
process = subprocess.Popen(['go', 'test', './internal/desktop', '-run', '^TestX11RealTargetPaste$', '-count=1'], cwd=sys.argv[1], env=env)
deadline = time.monotonic() + 45
while time.monotonic() < deadline:
    app.processEvents()
    if process.poll() is not None and editor.toPlainText() == 'ClipDeck target paste λ':
        break
    time.sleep(.01)
if process.poll() is None:
    process.kill()
assert process.wait() == 0, 'native backend paste failed'
assert editor.toPlainText() == 'ClipDeck target paste λ', repr(editor.toPlainText())
print('PASS: native backend restores original i3 target and pastes UTF-8 text')
PY
