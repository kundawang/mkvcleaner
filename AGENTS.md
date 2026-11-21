# AGENTS.md — mkvcleaner

## Project Overview

**mkvcleaner** is a Go CLI tool that bulk-remuxes MKV files to remove tracks (audio, subtitle) of unwanted languages. It uses `ffprobe` to inspect track metadata and `ffmpeg` to remux files, copying only the desired tracks into a new file.

Repository: `github.com/etu/mkvcleaner`

## Architecture

The project is a single Go package (`package main`) with no sub-packages. All source files live at the repository root.

### Source Files

| File | Purpose |
|---|---|
| `main.go` | Entry point. Parses CLI flags (`--langs`, `--audio-langs`, `--subtitle-langs`, `--keep-forced`, `--keep-commentary`, `--automatic`, `--version`), resolves file/directory arguments, and orchestrates processing with interactive confirmation. Declares the `version` package variable that `--version` prints, set at build time via `-ldflags -X main.version=...`. |
| `ffprobe.go` | `FFProbe`/`FFProbeStream` structs and methods. Shells out to `ffprobe` to identify streams in an MKV file. Provides filtering methods: `GetVideoTracks()`, `GetAudioTracks(languages)`, `GetSubtitleTracks(languages)`, `GetTracksStatus(tracksToKeep)`, `NeedsProcessing(tracksToKeep)`. |
| `strategy.go` | `TrackStrategy` and `FFProbe.PlanTracks()` — per-track-type filtering used when any of `--audio-langs`/`--subtitle-langs`/`--keep-forced`/`--keep-commentary` is set. Produces the kept-track list in original stream order plus disposition adjustments that make the output's default tracks match the language priorities. |
| `ffmpeg.go` | `FFMpeg` struct and methods. Builds and executes the `ffmpeg` remux command with `-c copy` and `-map` flags for selected tracks. Legacy mode maps tracks grouped by type (video, audio, subtitles); strategy mode maps `mapOrder` (original stream order) and appends `-disposition` args. Handles shell escaping of file paths via `shellescape`. |
| `fileutils.go` | `findFilesInDirectory()` — recursively walks a directory tree and returns all `.mkv` file paths. `copyFilePermissions(srcPath, dstPath)` — copies owner, group, and mode from one file onto another. |
| `ffmpeg_test.go` | Unit tests for `FFMpeg.FormatCommandParts()`, covering various path formats, special characters, and track combinations. |
| `ffprobe_test.go` | Unit tests for the legacy track filtering (`ffprobe.go`) built on JSON fixtures. |
| `strategy_test.go` | Unit tests for `FFProbe.PlanTracks()` and strategy-mode command construction, built on a fixed probe fixture with dual audio tracks, three subtitles, and a forced flag. |
| `fileutils_test.go` | Unit tests for `findFilesInDirectory()` and `copyFilePermissions()`. |

### Key Data Flow

1. CLI args are parsed; directories are expanded to `.mkv` file lists (`fileutils.go`).
2. Each file is identified with `ffprobe` (`ffprobe.go`) to discover all streams.
3. Streams are filtered by language against the wanted languages list — either the single shared `--langs` list (legacy mode) or the per-track-type `TrackStrategy` (`strategy.go`) when any of `--audio-langs`/`--subtitle-langs`/`--keep-forced`/`--keep-commentary` is set.
4. If all audio tracks would be removed, all audio tracks are kept instead (safety behavior, both modes).
5. Subtitle tracks matching no wanted language are simply removed (no safety fallback).
6. All video tracks are always kept.
7. A table is printed showing which tracks will be kept/removed.
8. If changes are needed and the user confirms (or `--automatic` is set), `ffmpeg` remuxes to a temp file. The original file's owner, group, and mode are then copied onto that temp file (`copyFilePermissions` in `fileutils.go`) before it atomically replaces the original via rename — if that permission copy fails, the temp file is discarded and the original is left untouched.

### Strategy Mode Details

- Audio and subtitle language lists are priority-ordered: every matching track is kept, and the kept track matching the highest-priority language becomes the output's default track (stale default flags on other kept tracks are cleared via `-disposition`). If no kept track of a type matches any wanted language (e.g. the audio safety fallback), that type's dispositions are left untouched.
- Tracks are mapped in original stream order (legacy mode maps grouped by type: video, audio, subtitles).
- A stream with no language tag is treated as `und` (undefined).
- Forced subtitles (`disposition.forced`) are kept with `--keep-forced`; commentary audio (`disposition.comment` or a "commentary" title) is kept with `--keep-commentary`.
- If the `.tmp.`-prefixed output file already exists (leftover from an interrupted run), the file is skipped with a message. If `ffmpeg` fails, the partial temp file is removed and the original is left untouched.

### CLI Interface

```
./mkvcleaner [--langs=und,eng,swe,jap,jpn] [--audio-langs=jpn] [--subtitle-langs=zho,eng] [--keep-forced] [--keep-commentary] [--automatic] <path>...
```

- `--langs`: Comma-separated list of wanted language codes (default: `und,eng,swe,jap,jpn`).
- `--audio-langs`: Comma-separated priority list of wanted audio languages (default: the `--langs` list). Enables strategy mode.
- `--subtitle-langs`: Comma-separated priority list of wanted subtitle languages (default: the `--langs` list). Enables strategy mode.
- `--keep-forced`: Keep forced subtitle tracks regardless of language. Enables strategy mode.
- `--keep-commentary`: Keep commentary audio tracks regardless of language. Enables strategy mode.
- `--automatic`: Skip interactive confirmation prompts.
- `--version`: Print the version and exit.
- Positional args: Files or directories to process.

## Versioning

- Releases are tagged with bare semver (`MAJOR.MINOR.PATCH`, no `v` prefix — e.g. `1.1.0`), matching existing tags.
- `main.go` declares `var version = "dev"`, overridden at build time via `-ldflags "-X main.version=..."`.
- `make build` sets it from `git describe --tags --always --dirty`, so local/dev builds report something like `1.1.0-3-gabcdef-dirty`; a build from a clean tagged commit reports the bare tag.
- The Nix package (`flake.nix`) derives its own version from `self.shortRev` (or a date-based fallback when the tree is dirty/unavailable) and wires it through the same `ldflags` mechanism — this tracks the commit, not the semver tag, since flakes can't easily introspect git tags during evaluation.

## Dependencies

### Runtime

- `ffprobe` and `ffmpeg` (from the FFmpeg project) must be available on `PATH`.

### Go Modules

- `github.com/olekukonenko/tablewriter` — pretty-prints track status tables.
- `gopkg.in/alessio/shellescape.v1` — shell-escapes file paths for the ffmpeg command.

## Build & Development

### Nix

The project includes a `flake.nix` providing:
- A default package that builds the Go module and wraps the binary with `ffmpeg` on `PATH`.
- A dev shell with `ffmpeg`, `gnumake`, `delve`, `go`, and `gopls`.

### Makefile

- `make build` — compiles the binary to `./mkvcleaner`.
- `make test` — runs `go test`.

### Running Tests

```bash
go test
# or
make test
```

Tests are in `ffmpeg_test.go`, `ffprobe_test.go`, `strategy_test.go`, and `fileutils_test.go`, and cover the ffmpeg command construction and track filtering logic.

## Conventions & Guidelines

- **Language**: Go. Single `main` package, no sub-packages.
- **No frameworks**: Standard library + minimal dependencies only.
- **External tools**: The program shells out to `ffprobe` and `ffmpeg`; it does not use Go bindings.
- **Error handling**: Errors during processing of individual files are logged and skipped; the tool continues with remaining files.
- **File safety**: Remuxed output is written to a `.tmp.`-prefixed file, then atomically swapped via `os.Rename`. On failure, the temp file is cleaned up.
- **Permission preservation**: The remuxed file is chmod/chown'd to match the original's owner, group, and mode (`copyFilePermissions`) before the swap — `os.Chown` requires either running as the file's existing owner (setting the same uid/gid is permitted) or elevated privileges (e.g. root) for anything else.
- **Testing**: Table-driven tests using the standard `testing` package. No external test frameworks.

## Important Reminders for Agents

- **Keep AGENTS.md up to date.** When you add new files, change the architecture, modify CLI flags, add dependencies, or alter key behaviors, update this file to reflect those changes.
- **Keep README.md up to date.** When user-visible behavior changes (new flags, changed defaults, new dependencies, changed usage patterns), update `README.md` accordingly so end-user documentation stays accurate.
- **Run tests** (`make test` or `go test`) after making changes to verify nothing is broken.
- **Do not introduce new dependencies** without good reason. The project intentionally has minimal dependencies.

## Pre-Completion Checklist

Before finishing any task, review this checklist:

- [ ] **Tests pass** — ran `make test` or `go test` and all tests pass.
- [ ] **AGENTS.md is current** — if you added/removed files, changed CLI flags, modified dependencies, or altered key behaviors, update this file.
- [ ] **README.md is current** — if user-visible behavior changed (new flags, changed defaults, new usage patterns, new dependencies), update `README.md`.
- [ ] **No unnecessary dependencies added** — only add new Go modules if strictly required.
- [ ] **File safety preserved** — any new file operations still use temp files and atomic rename.
