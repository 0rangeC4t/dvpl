# DVPL

A Go command-line tool to compress and decompress `.dvpl` files (the Dava Engine format used by World of Tanks Blitz), with a terminal UI (TUI). Vietnamese: [README.md](README.md).

## Install

Requires Go 1.22 or newer.

```sh
git clone https://github.com/0rangeC4t/dvpl.git
cd dvpl
go build -o dvpl ./cmd/dvpl     # build for this machine
make build                      # cross-platform builds into dist/
```

`make build` produces `dist/dvpl-linux-amd64`, `dist/dvpl-linux-arm64`, `dist/dvpl-windows-amd64.exe` and `dist/dvpl-linux-amd64-purego`.

Both build modes (`cgo` and `purego`) currently use the same pure Go backend (`pierrec/lz4`). A separate cgo backend is planned.

## Usage

```sh
dvpl compress -level 2 assets/          # compress a directory
dvpl decompress assets.dvpl             # decompress one file
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt                          # compresses; a .dvpl file is decompressed
dvpl tui                                # terminal UI
```

Running `dvpl` with no arguments in an interactive terminal also opens the TUI. `c`/`d` and `compress`/`decompress` (plus the aliases `encrypt`/`decrypt`) are equivalent. **Flags must come before the path.**

| Flag | Default | Meaning |
|---|---:|---|
| `-o` | next to input | Output directory (subdirectory layout is preserved) |
| `-level` | `2` | `0` store, `1` LZ4, `2` LZ4HC |
| `-f` | off | Keep the chosen compression type even if the result is not smaller |
| `-filter` | — | Only process files whose name matches a pattern (comma separated) |
| `-ignore` | — | Skip files whose name matches a pattern |
| `-ignore-compress` | — | Store matching files without compression |
| `-workers` | `2` | Number of workers, capped at the CPU count |
| `-trust-crc` | off | Skip CRC verification when decompressing (prints a warning) |
| `-delete-original` | off | Delete the source file after a successful conversion |
| `-q`, `-v` | off | Quiet per-file output / verbose byte counts |

Patterns match the **file name** (not the path) using `filepath.Match` syntax. Existing output files are overwritten.

| Environment variable | Meaning |
|---|---|
| `DVPL_LEVEL` | Compression level `0`–`2`; the `-level` flag takes priority |
| `DVPL_WORKERS` | Positive worker count; the `-workers` flag takes priority |
| `DVPL_LANG` | TUI language: `vi` or `en` |

Exit codes: `0` success, `1` some files failed, `2` configuration/argument error, `130` interrupted. One failing file does not stop the batch.

## Data safety

- Originals are **kept by default**; they are deleted only with `-delete-original`.
- Files are written to a `*.dvpltmp` temporary file in the same directory and then renamed, so output is never half-written.
- Symlinks are skipped. A warning is printed if the output directory is inside the input directory.
- Magic, sizes and the payload CRC32 are checked when decompressing.

## TUI

**How to open it:**

```sh
dvpl                  # no arguments, in an interactive terminal
dvpl tui              # always opens the TUI
dvpl tui --lang en    # open in English
dvpl --lang en        # also opens the TUI (when run in an interactive terminal)
./dist/dvpl-linux-amd64 tui    # if you run a build from dist/
```

`dvpl` with no arguments opens the TUI only when input is a real terminal; in a pipe or script it reports a missing path (exit code `2`), so use `dvpl tui` to be sure.

Pick compress/decompress, enter source and destination, set the level `0/1/2` (left/right, `+/-` or type a number), workers (defaults to the CPU count; 1 to 64 can be entered but the effective value is capped at the CPU count), and the keep/delete original, force and trust-crc options. The progress screen shows a progress bar, counters, speed and elapsed time; the scrollable error pane keeps the last 500 errors. On the result screen press `s` to save the error list to `errors.log`.

The UI defaults to Vietnamese. Switch with `dvpl tui --lang en`, `DVPL_LANG=en`, or the `L` key in the form. Use Tab/arrows to move, Enter to select, Esc to go back/cancel, `q` or Ctrl+C to quit.

To add a language, create `internal/tui/lang/<code>.json` with the same keys as `vi.json` and `en.json` (JSON files are embedded automatically), add the code to `lang.Set`/`lang.Other` and to the `--lang`/`DVPL_LANG` check in the CLI, then update the key-parity test.

## DVPL format

```
[payload (compressed or raw)][20-byte footer, little-endian]

footer:
  uint32  original size
  uint32  payload size
  uint32  CRC32 (IEEE) of the payload
  uint32  compression type
  4 bytes "DVPL"
```

| Type | Meaning | Read | Write |
|---:|---|:--:|:--:|
| `0` | stored | yes | yes |
| `1` | LZ4 block | yes | yes |
| `2` | LZ4HC block | yes | yes |
| `4` | single LZ4 block with a 28-byte header, payload CRC ignored | yes | no |

Any other value is rejected. If compression does not make the file smaller (and `-f` is not set) the tool writes type `0`. The size limit is `0x7E000000` bytes. The type labels in the footer have not been verified against real game files.

## Tests

```sh
go test ./...
go test -tags purego ./...
```

## License and credits

Apache-2.0, see `LICENSE` and `NOTICE`. The project consulted [qirashi/dvpl_go](https://github.com/qirashi/dvpl_go) (Apache-2.0) and [Tankerch/dvpl_converter](https://github.com/Tankerch/dvpl_converter) (MIT) as references; no source text was copied. Dependencies: [pierrec/lz4](https://github.com/pierrec/lz4) (BSD-3-Clause), and Charmbracelet's Bubble Tea, Bubbles and Lip Gloss (MIT).

This is an unofficial community tool and i
s not affiliated with Wargaming.
