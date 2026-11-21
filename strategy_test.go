package main

import (
	"reflect"
	"testing"
)

// movieStreamsJSON is a fixed ffprobe result for a movie with dual audio
// tracks (Japanese main, English dub), three subtitle tracks (Chinese,
// English — marked default in the input, and a forced Japanese
// signs/songs track).
const movieStreamsJSON = `{
	"streams": [
		{"index": 0, "codec_name": "hevc", "codec_type": "video", "tags": {"language": "und"}, "disposition": {"default": 1}},
		{"index": 1, "codec_name": "flac", "codec_type": "audio", "tags": {"language": "jpn", "title": "Main Audio"}, "disposition": {"default": 1}},
		{"index": 2, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "eng", "title": "English Dub"}, "disposition": {"default": 0}},
		{"index": 3, "codec_name": "subrip", "codec_type": "subtitle", "tags": {"language": "zho", "title": "Chinese"}, "disposition": {"default": 0}},
		{"index": 4, "codec_name": "subrip", "codec_type": "subtitle", "tags": {"language": "eng", "title": "English"}, "disposition": {"default": 1}},
		{"index": 5, "codec_name": "subrip", "codec_type": "subtitle", "tags": {"language": "jpn", "title": "Signs & Songs"}, "disposition": {"default": 0, "forced": 1}}
	]
}`

// TestPlanTracks verifies track selection, map order, and default-track
// dispositions against the fixed movie fixture.
func TestPlanTracks(t *testing.T) {
	tests := []struct {
		name             string
		json             string
		strategy         TrackStrategy
		wantMapOrder     []int
		wantDispositions []StreamDisposition
	}{
		{
			name: "japanese audio, chinese and english subtitles, forced kept",
			json: movieStreamsJSON,
			strategy: TrackStrategy{
				AudioLanguages:    []string{"jpn", "jap"},
				SubtitleLanguages: []string{"zho", "chi", "eng"},
				KeepForced:        true,
			},
			// English dub dropped; forced Japanese signs kept despite jpn
			// not being in the subtitle list. Map order follows the
			// original stream order.
			wantMapOrder: []int{0, 1, 3, 4, 5},
			// Japanese audio stays default; the default subtitle flag
			// moves from the English track to the higher-priority
			// Chinese track.
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
				{StreamSpec: "s:0", Value: "default"},
				{StreamSpec: "s:1", Value: "-default"},
			},
		},
		{
			name: "forced subtitles dropped without keep-forced",
			json: movieStreamsJSON,
			strategy: TrackStrategy{
				AudioLanguages:    []string{"jpn", "jap"},
				SubtitleLanguages: []string{"zho", "chi", "eng"},
				KeepForced:        false,
			},
			wantMapOrder: []int{0, 1, 3, 4},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
				{StreamSpec: "s:0", Value: "default"},
				{StreamSpec: "s:1", Value: "-default"},
			},
		},
		{
			name: "keeping every audio and subtitle language keeps all tracks",
			json: movieStreamsJSON,
			strategy: TrackStrategy{
				AudioLanguages:    []string{"jpn", "eng"},
				SubtitleLanguages: []string{"zho", "eng", "jpn"},
			},
			wantMapOrder: []int{0, 1, 2, 3, 4, 5},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
				{StreamSpec: "s:0", Value: "default"},
				{StreamSpec: "s:1", Value: "-default"},
			},
		},
		{
			name: "no matching audio language keeps all audio and leaves audio dispositions alone",
			json: movieStreamsJSON,
			strategy: TrackStrategy{
				AudioLanguages:    []string{"deu"},
				SubtitleLanguages: []string{"eng"},
			},
			// Safety fallback: both audio tracks kept.
			wantMapOrder: []int{0, 1, 2, 4},
			// No audio disposition flags (fallback leaves them
			// untouched); the English subtitle is already default.
			wantDispositions: []StreamDisposition{
				{StreamSpec: "s:0", Value: "default"},
			},
		},
		{
			name: "commentary audio dropped by default",
			json: `{
				"streams": [
					{"index": 0, "codec_name": "hevc", "codec_type": "video"},
					{"index": 1, "codec_name": "flac", "codec_type": "audio", "tags": {"language": "jpn"}, "disposition": {"default": 1}},
					{"index": 2, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "eng", "title": "Commentary"}, "disposition": {"comment": 1}}
				]
			}`,
			strategy: TrackStrategy{
				AudioLanguages: []string{"jpn"},
				KeepCommentary: false,
			},
			wantMapOrder: []int{0, 1},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
			},
		},
		{
			name: "commentary audio kept with keep-commentary",
			json: `{
				"streams": [
					{"index": 0, "codec_name": "hevc", "codec_type": "video"},
					{"index": 1, "codec_name": "flac", "codec_type": "audio", "tags": {"language": "jpn"}, "disposition": {"default": 1}},
					{"index": 2, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "eng", "title": "Commentary"}, "disposition": {"comment": 1}}
				]
			}`,
			strategy: TrackStrategy{
				AudioLanguages: []string{"jpn"},
				KeepCommentary: true,
			},
			wantMapOrder: []int{0, 1, 2},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
			},
		},
		{
			name: "commentary detected by title alone",
			json: `{
				"streams": [
					{"index": 0, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "eng", "title": "Director's Commentary"}},
					{"index": 1, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "jpn"}}
				]
			}`,
			strategy: TrackStrategy{
				AudioLanguages: []string{"jpn"},
				KeepCommentary: true,
			},
			wantMapOrder: []int{0, 1},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:1", Value: "default"},
			},
		},
		{
			name: "missing language tag is treated as undefined",
			json: `{
				"streams": [
					{"index": 0, "codec_name": "hevc", "codec_type": "video"},
					{"index": 1, "codec_name": "aac", "codec_type": "audio"},
					{"index": 2, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "jpn"}}
				]
			}`,
			strategy: TrackStrategy{
				AudioLanguages: []string{"und", "jpn"},
			},
			wantMapOrder: []int{0, 1, 2},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
			},
		},
		{
			name: "duplicate language tracks are all kept, first becomes default",
			json: `{
				"streams": [
					{"index": 0, "codec_name": "hevc", "codec_type": "video"},
					{"index": 1, "codec_name": "flac", "codec_type": "audio", "tags": {"language": "jpn"}},
					{"index": 2, "codec_name": "aac", "codec_type": "audio", "tags": {"language": "jpn"}, "disposition": {"default": 1}}
				]
			}`,
			strategy: TrackStrategy{
				AudioLanguages: []string{"jpn"},
			},
			wantMapOrder: []int{0, 1, 2},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
				{StreamSpec: "a:1", Value: "-default"},
			},
		},
		{
			name: "subtitle-only file",
			json: `{
				"streams": [
					{"index": 0, "codec_name": "subrip", "codec_type": "subtitle", "tags": {"language": "eng"}},
					{"index": 1, "codec_name": "subrip", "codec_type": "subtitle", "tags": {"language": "ger"}}
				]
			}`,
			strategy: TrackStrategy{
				AudioLanguages:    []string{"jpn"},
				SubtitleLanguages: []string{"eng"},
			},
			wantMapOrder: []int{0},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "s:0", Value: "default"},
			},
		},
		{
			name: "no matching subtitle language removes all subtitles",
			json: movieStreamsJSON,
			strategy: TrackStrategy{
				AudioLanguages:    []string{"jpn"},
				SubtitleLanguages: []string{"deu"},
			},
			wantMapOrder: []int{0, 1},
			wantDispositions: []StreamDisposition{
				{StreamSpec: "a:0", Value: "default"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ffprobe := newFFProbe(t, tt.json)

			mapOrder, dispositions := ffprobe.PlanTracks(tt.strategy)

			if !reflect.DeepEqual(mapOrder, tt.wantMapOrder) {
				t.Errorf("expected map order %v, got %v", tt.wantMapOrder, mapOrder)
			}
			if !reflect.DeepEqual(dispositions, tt.wantDispositions) {
				t.Errorf("expected dispositions %v, got %v", tt.wantDispositions, dispositions)
			}
		})
	}
}

// TestPlanTracksCommand verifies the full ffmpeg command built from the
// fixed movie fixture: japanese audio, chinese and english subtitles,
// forced subtitles kept.
func TestPlanTracksCommand(t *testing.T) {
	ffprobe := newFFProbe(t, movieStreamsJSON)

	strategy := TrackStrategy{
		AudioLanguages:    []string{"jpn", "jap"},
		SubtitleLanguages: []string{"zho", "chi", "eng"},
		KeepForced:        true,
	}

	mapOrder, dispositions := ffprobe.PlanTracks(strategy)

	ffmpeg := &FFMpeg{
		inputFilePath: "/path/to/input/movie.mkv",
		mapOrder:      mapOrder,
		dispositions:  dispositions,
	}

	want := []string{
		"ffmpeg",
		"-i",
		"/path/to/input/movie.mkv",
		"-c",
		"copy",
		"-map 0:0",
		"-map 0:1",
		"-map 0:3",
		"-map 0:4",
		"-map 0:5",
		"-disposition:a:0 default",
		"-disposition:s:0 default",
		"-disposition:s:1 -default",
		"/path/to/input/.tmp.movie.mkv",
	}

	got := ffmpeg.FormatCommandParts()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected command parts %v, got %v", want, got)
	}

}

// TestFormatCommandParts_legacyUnchanged verifies that the legacy code
// path (no mapOrder, no dispositions) produces the same grouped command
// as before.
func TestFormatCommandParts_legacyUnchanged(t *testing.T) {
	ffmpeg := &FFMpeg{
		inputFilePath:  "/path/to/input/file.mkv",
		audioTracks:    []int{1, 2},
		subtitleTracks: []int{3, 4},
		videoTracks:    []int{0},
	}

	want := []string{
		"ffmpeg",
		"-i",
		"/path/to/input/file.mkv",
		"-c",
		"copy",
		"-map 0:0",
		"-map 0:1",
		"-map 0:2",
		"-map 0:3",
		"-map 0:4",
		"/path/to/input/.tmp.file.mkv",
	}

	got := ffmpeg.FormatCommandParts()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected command parts %v, got %v", want, got)
	}
}

func TestSplitLanguages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "plain list", input: "jpn,eng", want: []string{"jpn", "eng"}},
		{name: "empty entries and spaces dropped", input: "jpn, ,eng,", want: []string{"jpn", "eng"}},
		{name: "empty string", input: "", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLanguages(tt.input)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
