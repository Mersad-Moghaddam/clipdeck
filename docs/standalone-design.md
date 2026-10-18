# Standalone design provenance

The standalone popup uses Qt Quick, with no imports from Quickshell or Omarchy.
The existing plugin overlay is not modified.

## Authoritative references

- Omarchy `shell/Commons/Style.qml`: 12px base; caption/body-small/body/subtitle/title
  at 10/11/12/13/14px. `OMARCHY_MENU_FONT` controls the menu family.
- ClipDeck `overlay/ClipDeck.qml`: header, search, categories, two-column
  history/inspector, selection accent, plain-text content and action footer.
- Active Omarchy palette: `$XDG_STATE_HOME/omarchy/current/theme/colors.toml`
  and `shell.toml`, with `$XDG_CONFIG_HOME/omarchy/shell.toml` overrides.
  Files are bounded, parsed as TOML, and never executed. Invalid colors and
  out-of-range sizes retain safe defaults. The palette is resolved at launch.

## Fonts

Regular and bold **JetBrainsMono Nerd Font** are included in the wheel and loaded
privately into Qt. They are not installed into the user's font directories.
Source: Arch `ttf-jetbrains-mono-nerd-basic`, upstream
<https://github.com/ryanoasis/nerd-fonts> / JetBrains Mono. The accompanying
`desktop/clipdeck_ui/fonts/OFL.txt` contains the copyright and SIL Open Font
License 1.1. Font files retain their original names and are not modified.
An explicit `OMARCHY_MENU_FONT` still takes precedence; Qt resolves unavailable
custom families using its normal fallback behavior.

Bundled file SHA-256 provenance:

```text
1c680e8cde9fcf8b88a5605ce8d1fb94dd3fb15841f7ca7bf4c55664855e5611  JetBrainsMonoNerdFont-Regular.ttf
e490660ad75e0b152c93b1604c2ea1a4ea675f1b8a3fe0d005b1998568f99f1f  JetBrainsMonoNerdFont-Bold.ttf
```

Python 3.10+ and PySide6-Essentials 6.6+ are required. Python 3.10 uses `tomli`;
newer Python uses the standard-library TOML parser. PySide6 is installed in a
private virtual environment by the optional GUI installer, never system pip.
