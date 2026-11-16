# DVPL

DVPL is a Go command-line tool and terminal UI for compressing and decompressing `.dvpl` files used by Dava Engine games, including World of Tanks Blitz. It supports Linux amd64/arm64 and Windows amd64/386 without CGO. The interface is available in Vietnamese and English.

Tiếng Việt: [README.vi.MD](README.vi.MD).

## Install

### Arch Linux

From the project source, run:

```sh
cd packaging/arch
makepkg -si
```

### Linux and Windows binaries

Download the archive for your operating system and CPU from [GitHub Releases](https://github.com/0rangeC4t/dvpl/releases/latest), then extract it. The archive includes the executable, both READMEs, `LICENSE`, and `NOTICE`. Run `dvpl` on Linux or `dvpl.exe` on Windows; add the extracted directory to `PATH` if you want to call it from any directory.

| Platform | Archive |
|---|---|
| Linux x86-64 | `dvpl-<version>-linux-amd64.tar.gz` |
| Linux ARM64 | `dvpl-<version>-linux-arm64.tar.gz` |
| Linux x86-64, purego | `dvpl-<version>-linux-amd64-purego.tar.gz` |
| Windows 64-bit | `dvpl-<version>-windows-amd64.zip` |
| Windows 32-bit | `dvpl-<version>-windows-386.zip` |

Windows may show a SmartScreen prompt because the executable is not code-signed. Run it from Windows Terminal or PowerShell. TUI Vietnamese text requires a terminal with UTF-8 support.

## CLI

```sh
dvpl compress -level 2 assets/             # compress a directory
dvpl decompress assets.dvpl                # decompress a file
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt                             # compress; a .dvpl input is decompressed
dvpl --version
```

The commands `c`/`d` and `encrypt`/`decrypt` are aliases for `compress`/`decompress`. With no command, a `.dvpl` input is decompressed and any other input is compressed. Flags must come before the input path. Directory inputs are processed recursively; output subdirectories are preserved when `-o` is set. Without `-o`, each output is placed beside its input. Existing outputs are replaced. Files are written to a same-directory temporary file and renamed into place. Symlinks and temporary `.dvpltmp` files are skipped. The program warns if the output directory is inside the input directory. Originals are kept unless `-delete-original` is set.

| Flag | Default | Meaning |
|---|---:|---|
| `-o` | beside input | Output directory |
| `-level` | `2` | Compression effort: `0` store, `1` LZ4, `2` LZ4HC |
| `-f` | off | Keep the selected compression type even when output is not smaller |
| `-filter` | — | Process only matching file names; comma-separated `filepath.Match` patterns |
| `-ignore` | — | Skip matching file names |
| `-ignore-compress` | — | Store matching files without compression |
| `-workers` | `2` | Worker count, capped at the available CPU count |
| `-trust-crc` | off | Skip payload CRC comparison when decompressing |
| `-delete-original` | off | Delete a source only after successful conversion |
| `-q` | off | Suppress per-file messages |
| `-v` | off | Show per-file byte counts and output paths |

`DVPL_LEVEL` and `DVPL_WORKERS` set defaults when their flags are omitted. `DVPL_LANG` selects the TUI language (`vi` or `en`). Exit codes are `0` for success, `1` when one or more files fail, `2` for argument or job errors, and `130` when interrupted. DVPL magic, sizes and payload CRC are checked while decompressing, unless `-trust-crc` skips the CRC comparison. A file failure does not stop the rest of a batch.

The format has a 20-byte little-endian footer containing original size, payload size, payload CRC32-IEEE, type, and `DVPL` magic. Readers accept types `0` (raw), `1` and `2` (LZ4 blocks), and `4` (one LZ4 block with a 28-byte header; payload CRC is not checked). Types `0`–`2` can be written; type `4` is read-only. Other types are rejected. If compression is not smaller, the output is stored as type `0` unless `-f` is used. The maximum input size is `0x7E000000` bytes. The type labels have not been verified against real game files.

## TUI

Run `dvpl tui` to open the interface explicitly. Running `dvpl` with no arguments opens it when standard input is an interactive terminal. Choose a source file or directory with the browser; choose an output directory, compression level and worker count, then select **Start**. The progress view shows counts, speed, elapsed time and estimated time remaining. It keeps the most recent 500 errors while running and shows the complete error list on the result screen. Press `s` on the result screen to save errors to `errors.log`.In the TUI the worker count defaults to the number of CPU threads (1–64); your last choice is remembered between runs.

Use `dvpl tui --lang en`, `dvpl --lang en` in an interactive terminal, `DVPL_LANG=en`, or `L` in the form to select English. Vietnamese is the default.

| Key | Form | Browser | Progress / Result |
|---|---|---|---|
| `Tab` / `↓` | Move to next field | Move selection down | Scroll errors down |
| `Shift+Tab` / `↑` | Move to previous field | Move selection up | Scroll errors up |
| `←` / `→` | Switch mode, level or worker count according to focus | — | — |
| `+` / `-` | Adjust focused level or worker count | — | — |
| `Space` | Switch mode or toggle focused option | Select current directory | — |
| `Enter` | Select, open browser, or start the job | Open directory or choose file | Result: return to form |
| `Backspace` | — | Go to parent directory; at a Windows drive root, list drives | — |
| `Esc` | Move focus back | Close browser | Progress: cancel; result: return to form |
| `L` | Switch Vietnamese/English when not editing text | — | — |
| `PgUp` / `PgDn` | — | — | Scroll errors |
| `s` | — | Select current directory | Result: save `errors.log` |
| `q` / `Ctrl+C` | Quit (except `q` while editing a text field) | — | Quit |

## Build from source

Requires Go 1.22 or newer and Make. From the repository root:

```sh
make build VERSION=0.1.0
```

This builds Linux amd64/arm64, Windows amd64/386, and Linux amd64 purego binaries into `dist/`. Before building Windows binaries for a version, regenerate their icon and version resources with `make winres VERSION=0.1.0` (requires `llvm-rc` and `llvm-cvtres`). `make checksums VERSION=0.1.0` builds all five binaries and writes checksums for the binaries in `dist/SHA256SUMS`.

## License

DVPL is licensed under Apache-2.0; see `LICENSE` and `NOTICE`. The project consulted qirashi/dvpl_go (Apache-2.0) and Tankerch/dvpl_converter (MIT) as references. Dependencies include [pierrec/lz4](https://github.com/pierrec/lz4) (BSD-3-Clause) and Charmbracelet Bubble Tea, Bubbles and Lip Gloss (MIT). This is an unofficial community tool and is not affiliated with Wargaming.
