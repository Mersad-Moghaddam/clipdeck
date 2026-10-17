#!/bin/sh
# ClipDeck post-install hint. The daemon is a per-user service, so we cannot
# enable it from the package script — tell the user the commands to run.
if command -v systemctl >/dev/null 2>&1; then
cat <<'EOF'
ClipDeck installed. Enable the clipboard daemon for your user session with:

  systemctl --user enable --now clipdeckd.service

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
