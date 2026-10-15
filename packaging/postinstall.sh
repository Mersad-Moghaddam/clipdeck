#!/bin/sh
# ClipDeck post-install hint. The daemon is a per-user service, so we cannot
# enable it from the package script — tell the user the one command to run.
cat <<'EOF'
ClipDeck installed. Enable the clipboard daemon for your session with:

  systemctl --user enable --now clipdeckd.service

Verify everything with:

  clipdeck doctor
EOF
