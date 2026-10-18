#!/bin/sh
# The package installs a graphical-session dependency; never run a user
# service as root from a package hook.
if command -v systemctl >/dev/null 2>&1; then
cat <<'EOF'
ClipDeck installed. The daemon starts automatically at your next graphical login.
To start it in this session, click "Start & enable at login" in the ClipDeck plugin.

Verify everything with:

  clipdeck doctor
EOF
else
cat <<'EOF'
ClipDeck installed. Systemd was not detected; start the daemon in your
compositor autostart or session startup script:

  clipdeckd &

Verify everything with:

  clipdeck doctor
EOF
fi
