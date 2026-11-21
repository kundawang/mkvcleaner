[![Check](https://github.com/etu/mkvcleaner/actions/workflows/check.yml/badge.svg)](https://github.com/etu/mkvcleaner/actions/workflows/check.yml)
[![Update](https://github.com/etu/mkvcleaner/actions/workflows/update.yml/badge.svg)](https://github.com/etu/mkvcleaner/actions/workflows/update.yml)

# mkvcleaner
Tool to bulk-remux mkv-files from tracks of unwanted languages.

It will go through all files in a given directory or work on a single
file. Then it will identify all tracks in that file and remove all tracks
marked with a language that isn't in the list of wanted languages.

## Usage
You can specify either files or directories as arguments. The default set of
languages specified is `und,eng,swe,jap,jpn`. You can override this list
using the `--langs` flag.

```bash
./mkvcleaner [--langs=und,eng] [--automatic] path/to/directory [path/to/file.mkv] [path/to/other/directory] […]
```

By default it will prompt the user about the changes to a file to approve the
changes before it's executed. However, if the `--automatic` flag is provided
it will skip the confirmation.

### Per-track-type strategies

A single shared language list can't express "keep Japanese audio but only
Chinese and English subtitles". For that, there are per-track-type flags:

```bash
./mkvcleaner --audio-langs=jpn --subtitle-langs=zho,eng --keep-forced --automatic path/to/file.mkv
```

- `--audio-langs`: comma-separated priority list of wanted audio languages
  (defaults to the `--langs` list).
- `--subtitle-langs`: comma-separated priority list of wanted subtitle
  languages (defaults to the `--langs` list).
- `--keep-forced`: keep forced subtitle tracks (e.g. signs/songs) regardless
  of language.
- `--keep-commentary`: keep commentary audio tracks (detected via the
  comment disposition flag or a "commentary" title) regardless of language.

Using any of these flags switches to strategy mode, where tracks are mapped
in their original stream order and the output's default audio/subtitle
tracks are set to the kept track matching the highest-priority language
(any stale default flags on other tracks are cleared). Without these flags
the behavior is exactly as before.

Use `--version` to print the build's version and exit:

```bash
./mkvcleaner --version
```

## Notes about languages
Always keep `und` as a language. Lots of files out there with only one audio
or subtitle track got it's only track marked as `undefined` language, so you
probably always want to have `und` in your list of wanted languages. In
strategy mode, a track with no language tag at all is treated as `und`.

## Notes about audio tracks
If the script filter away all audio tracks, it will choose to not touch them
at all. Instead it will keep all the audio tracks. This applies in strategy
mode as well, and in that case the input's default-track flags are left
untouched.

## Notes about subtitle tracks
This script may remove all subtitle tracks if there's no tracks matching the
wanted languages list. So you may end up without subtitles.

## Notes about temporary files
The remuxed output is written to a `.tmp.`-prefixed file next to the
original before it replaces it. If ffmpeg fails, the partial temporary file
is removed and the original is left untouched. If a `.tmp.`-prefixed file
already exists (e.g. from an interrupted run), the file is skipped with a
message — remove the leftover file and run again.

## Notes about file permissions
The remuxed file's owner, group, and mode are copied from the original file
before it replaces it, so permissions aren't reset to defaults. Changing the
owner requires either already owning the file or running as root.

## Dependencies
It's a go program and it depends on `ffprobe` and `ffmpeg` from the `ffmpeg`
project. `ffprobe` is used to detect changes.

## Development

### Building
```bash
make build
```

`make build` embeds a version string derived from `git describe --tags` into
the binary (falls back to `dev` if that fails). Release builds should be
built from a tagged commit so `--version` reports a clean semver tag (e.g.
`1.2.0`) instead of a `<tag>-<commits>-g<sha>` description.

### Running Tests
```bash
make test
```

### Nix
A `flake.nix` is provided with a default package and a dev shell containing
all necessary tools (`ffmpeg`, `go`, `gopls`, `delve`, `gnumake`).

## Contributing
See [AGENTS.md](AGENTS.md) for architecture details, conventions, and
guidelines for working on this project.
