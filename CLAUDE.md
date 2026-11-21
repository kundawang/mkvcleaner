# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**mkvcleaner** is a Go CLI tool that bulk-remuxes MKV files to remove audio/subtitle tracks of unwanted languages. It uses `ffprobe` to inspect track metadata and `ffmpeg` to remux files, copying only the desired tracks into a new file.

Repository: `github.com/etu/mkvcleaner`

## Commands

```bash
make build   # compiles ./mkvcleaner, embedding version via `git describe --tags --always --dirty`
make test    # runs go test
go test      # equivalent to make test
go test -run TestName   # run a single test
```

Tests live in `ffmpeg_test.go`, `ffprobe_test.go`, `strategy_test.go`, and `fileutils_test.go`, and cover ffmpeg command construction and track filtering via table-driven tests using the standard `testing` package — no external test frameworks.

### Nix

`flake.nix` provides a default package (builds the Go module, wraps the binary with `ffmpeg` on `PATH`) and a dev shell with `ffmpeg`, `gnumake`, `delve`, `go`, `gopls`.

## Architecture

Single Go package (`package main`), no sub-packages, all source files at repo root.

| File | Purpose |
|---|---|
| `main.go` | Entry point. Parses CLI flags (`--langs`, `--audio-langs`, `--subtitle-langs`, `--keep-forced`, `--keep-commentary`, `--automatic`, `--version`), resolves file/directory arguments, orchestrates processing with interactive confirmation. Declares the `version` var printed by `--version`, set at build time via `-ldflags -X main.version=...`. |
| `ffprobe.go` | `FFProbe`/`FFProbeStream` structs and methods. Shells out to `ffprobe` to identify streams in an MKV file. Filtering methods: `GetVideoTracks()`, `GetAudioTracks(languages)`, `GetSubtitleTracks(languages)`, `GetTracksStatus(tracksToKeep)`, `NeedsProcessing(tracksToKeep)`. |
| `strategy.go` | `TrackStrategy` and `FFProbe.PlanTracks()` — per-track-type filtering used when any of `--audio-langs`/`--subtitle-langs`/`--keep-forced`/`--keep-commentary` is set. Produces the kept-track list in original stream order plus disposition adjustments that make the output's default tracks match the language priorities. |
| `ffmpeg.go` | `FFMpeg` struct/methods. Builds and executes the `ffmpeg` remux command with `-c copy` and `-map` flags for selected tracks. Legacy mode maps tracks grouped by type (video, audio, subtitles); strategy mode maps `mapOrder` (original stream order) and appends `-disposition` args. Handles shell escaping of file paths via `shellescape`. |
| `fileutils.go` | `findFilesInDirectory()` — recursively walks a directory tree and returns all `.mkv` file paths. `copyFilePermissions(srcPath, dstPath)` — copies owner, group, and mode from one file onto another. |
| `ffmpeg_test.go` | Unit tests for `FFMpeg.FormatCommandParts()`, covering path formats, special characters, and track combinations. |
| `ffprobe_test.go` | Unit tests for the legacy track filtering (`ffprobe.go`) built on JSON fixtures. |
| `strategy_test.go` | Unit tests for `FFProbe.PlanTracks()` and strategy-mode command construction, built on a fixed probe fixture with dual audio tracks, three subtitles, and a forced flag. |

### Key data flow

1. CLI args are parsed; directories are expanded to `.mkv` file lists (`fileutils.go`).
2. Each file is identified with `ffprobe` (`ffprobe.go`) to discover all streams.
3. Streams are filtered by language against the wanted languages list — either the single shared `--langs` list (legacy mode) or the per-track-type `TrackStrategy` (`strategy.go`) when any of `--audio-langs`/`--subtitle-langs`/`--keep-forced`/`--keep-commentary` is set.
4. **Safety behavior**: if all audio tracks would be removed, all audio tracks are kept instead (both modes).
5. Subtitle tracks matching no wanted language are simply removed — no safety fallback (a file can end up with no subtitles).
6. All video tracks are always kept.
7. A table is printed showing which tracks will be kept/removed.
8. If changes are needed and the user confirms (or `--automatic` is set), `ffmpeg` remuxes to a `.tmp.`-prefixed file. The original's owner, group, and mode are copied onto it (`copyFilePermissions`) — if that fails, the temp file is discarded and the original is left untouched. Otherwise the original is atomically replaced via rename (input renamed to `.rename-tmp`, output renamed to the input's original name, then the `.rename-tmp` file is removed).

### Strategy mode details

- Audio and subtitle language lists are priority-ordered: every matching track is kept, and the kept track matching the highest-priority language becomes the output's default track (stale default flags on other kept tracks are cleared via `-disposition`). If no kept track of a type matches any wanted language (e.g. the audio safety fallback), that type's dispositions are left untouched.
- Tracks are mapped in original stream order (legacy mode maps grouped by type: video, audio, subtitles).
- A stream with no language tag is treated as `und` (undefined).
- Forced subtitles (`disposition.forced`) are kept with `--keep-forced`; commentary audio (`disposition.comment` or a "commentary" title) is kept with `--keep-commentary`.
- If the `.tmp.`-prefixed output file already exists (leftover from an interrupted run), the file is skipped with a message. If `ffmpeg` fails, the partial temp file is removed and the original is left untouched.

### CLI interface

```
./mkvcleaner [--langs=und,eng,swe,jap,jpn] [--audio-langs=jpn] [--subtitle-langs=zho,eng] [--keep-forced] [--keep-commentary] [--automatic] <path>...
```

- `--langs`: comma-separated wanted language codes (default: `und,eng,swe,jap,jpn`). Keep `und` in the list — many single-track files mark their only track as `undefined`.
- `--audio-langs`: comma-separated priority list of wanted audio languages (default: the `--langs` list). Enables strategy mode.
- `--subtitle-langs`: comma-separated priority list of wanted subtitle languages (default: the `--langs` list). Enables strategy mode.
- `--keep-forced`: keep forced subtitle tracks regardless of language. Enables strategy mode.
- `--keep-commentary`: keep commentary audio tracks regardless of language. Enables strategy mode.
- `--automatic`: skip interactive per-file confirmation prompts.
- `--version`: print the version and exit.
- Positional args: files or directories (directories are scanned recursively for `.mkv` files).

## Versioning

- Releases are tagged with bare semver (`MAJOR.MINOR.PATCH`, no `v` prefix — e.g. `1.1.0`).
- `main.go` declares `var version = "dev"`, overridden via `-ldflags -X main.version=...` at build time.
- `make build` sources it from `git describe --tags --always --dirty`.
- `flake.nix` sources its own version from `self.shortRev` (commit-based, not tag-based — flakes can't easily read git tags during evaluation) and wires it through the same `ldflags`.

## Dependencies

- Runtime: `ffprobe` and `ffmpeg` (from FFmpeg) must be on `PATH`.
- Go modules: `github.com/olekukonenko/tablewriter` (track status tables), `gopkg.in/alessio/shellescape.v1` (shell-escaping file paths for the ffmpeg command).
- The project intentionally keeps dependencies minimal — don't add new ones without good reason.

## Conventions

- No Go bindings for ffmpeg/ffprobe — the program always shells out to the CLI tools.
- Errors during processing of an individual file are logged and that file is skipped; the tool continues with remaining files.
- File safety: remuxed output is written to a `.tmp.`-prefixed file, then atomically swapped via `os.Rename`. On failure, the temp file is cleaned up. Preserve this pattern for any new file operations.
- Permission preservation: the remuxed file is chmod/chown'd (`copyFilePermissions`) to match the original before the swap. `os.Chown` needs either the caller to already own the file (same uid/gid is fine) or root — Unix-only (`syscall.Stat_t`), no Windows support.
- Keep `README.md` in sync when user-visible behavior changes (new flags, changed defaults, new dependencies).
- `AGENTS.md` in the repo root contains equivalent guidance for other coding agents — keep it in sync with this file if either is updated.
