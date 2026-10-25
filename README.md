# Blaze

A small endless-runner in the spirit of the Chrome dino game, except you play a campfire.

- Jump over wooden obstacles and duck under branches. Each hit costs a flame, and the run ends when the last one goes out.
- You start with 3 flames. Up to 3 spare flames trail behind the fire.
- Collect flames as you go. Every 10 small flames merge into a big one and give you a one-time blue shield.
- Collect 100 flames to form the huge flame and win.
- Light and dark mode, toggled with the flame button on the main menu.

## Controls

| Key | Action |
| --- | --- |
| Space, Up arrow or click | Jump (hold for a higher jump) |
| Down arrow | Duck, or drop faster in the air |
| Esc / P | Pause |
| D | Toggle dark mode (menu or pause) |
| F11 | Fullscreen |

The best score and the dark-mode setting are saved in `%AppData%\Blaze\settings.json`.

## Building

Needs [Go](https://go.dev/dl/) 1.22 or newer. Run `build.bat`, or:

```
go build -trimpath -ldflags "-s -w -H=windowsgui" -o Blaze.exe .
```

Built with [Ebitengine](https://ebitengine.org). `rsrc_windows_amd64.syso` embeds the app icon (`icon.ico`) into the exe.
`shots.go` holds test-only helpers and is only compiled with `-tags shots`.
