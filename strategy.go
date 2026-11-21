package main

import (
	"fmt"
	"strings"
)

// TrackStrategy configures per-track-type filtering. It is used instead of
// the single shared language list when any of the per-type flags
// (--audio-langs, --subtitle-langs, --keep-forced, --keep-commentary) is
// given.
type TrackStrategy struct {
	// AudioLanguages and SubtitleLanguages are priority-ordered lists of
	// wanted languages: every matching track is kept, and the first list
	// entry that matches a kept track decides the output's default track.
	AudioLanguages    []string
	SubtitleLanguages []string

	// KeepForced keeps forced subtitle tracks regardless of language.
	KeepForced bool

	// KeepCommentary keeps commentary audio tracks regardless of language.
	KeepCommentary bool
}

// PlanTracks computes which tracks to keep and which disposition
// adjustments are needed so the output's default tracks match the language
// priorities. The returned map order follows the original stream order.
func (ffprobe *FFProbe) PlanTracks(strategy TrackStrategy) (mapOrder []int, dispositions []StreamDisposition) {
	audioTracks := ffprobe.strategyAudioTracks(strategy)
	subtitleTracks := ffprobe.strategySubtitleTracks(strategy)

	keep := map[int]bool{}
	for _, index := range audioTracks {
		keep[index] = true
	}
	for _, index := range subtitleTracks {
		keep[index] = true
	}

	streamsByIndex := map[int]FFProbeStream{}
	for _, stream := range ffprobe.Streams {
		streamsByIndex[stream.Index] = stream

		// Video tracks are always kept.
		if stream.CodecType == "video" {
			keep[stream.Index] = true
		}
	}

	// Build the map list in original stream order.
	for _, stream := range ffprobe.Streams {
		if keep[stream.Index] {
			mapOrder = append(mapOrder, stream.Index)
		}
	}

	// Pick the default tracks: the kept track matching the
	// highest-priority language wins, ties broken by stream order. When no
	// kept track matches any wanted language (e.g. the audio safety
	// fallback kicked in), the input's dispositions are left untouched.
	defaultAudio, hasDefaultAudio := pickDefaultTrack(ffprobe.Streams, audioTracks, strategy.AudioLanguages)
	defaultSubtitle, hasDefaultSubtitle := pickDefaultTrack(ffprobe.Streams, subtitleTracks, strategy.SubtitleLanguages)

	// Walk the output in map order, tracking per-type output stream
	// indices, and emit disposition adjustments where the input's default
	// flags don't match the language priorities.
	audioIndex, subtitleIndex := 0, 0
	for _, index := range mapOrder {
		stream := streamsByIndex[index]

		switch stream.CodecType {
		case "audio":
			if hasDefaultAudio {
				dispositions = appendDisposition(dispositions, "a", audioIndex, index == defaultAudio, stream.Disposition.Default == 1)
			}
			audioIndex++
		case "subtitle":
			if hasDefaultSubtitle {
				dispositions = appendDisposition(dispositions, "s", subtitleIndex, index == defaultSubtitle, stream.Disposition.Default == 1)
			}
			subtitleIndex++
		}
	}

	return mapOrder, dispositions
}

// strategyAudioTracks returns the indices of the audio tracks to keep. As
// with the legacy behavior, if filtering would remove every audio track,
// all audio tracks are kept instead.
func (ffprobe *FFProbe) strategyAudioTracks(strategy TrackStrategy) []int {
	var tracks []int

	for _, stream := range ffprobe.Streams {
		if stream.CodecType != "audio" {
			continue
		}

		if matchesAnyLanguage(languageOf(stream), strategy.AudioLanguages) ||
			(strategy.KeepCommentary && isCommentary(stream)) {
			tracks = append(tracks, stream.Index)
		}
	}

	if len(tracks) == 0 {
		// Safety fallback: never remove all audio tracks.
		for _, stream := range ffprobe.Streams {
			if stream.CodecType == "audio" {
				tracks = append(tracks, stream.Index)
			}
		}
	}

	return tracks
}

// strategySubtitleTracks returns the indices of the subtitle tracks to
// keep. There is no safety fallback for subtitles: a file may end up with
// no subtitles at all.
func (ffprobe *FFProbe) strategySubtitleTracks(strategy TrackStrategy) []int {
	var tracks []int

	for _, stream := range ffprobe.Streams {
		if stream.CodecType != "subtitle" {
			continue
		}

		if matchesAnyLanguage(languageOf(stream), strategy.SubtitleLanguages) ||
			(strategy.KeepForced && stream.Disposition.Forced == 1) {
			tracks = append(tracks, stream.Index)
		}
	}

	return tracks
}

// pickDefaultTrack returns the kept track matching the highest-priority
// language; ties between tracks matching the same language are broken by
// original stream order.
func pickDefaultTrack(streams []FFProbeStream, kept []int, languages []string) (int, bool) {
	for _, language := range languages {
		for _, stream := range streams {
			if containsInt(kept, stream.Index) && matchesAnyLanguage(languageOf(stream), []string{language}) {
				return stream.Index, true
			}
		}
	}

	return 0, false
}

// appendDisposition records a disposition adjustment for an output stream
// when one is needed: the chosen default track is always marked default,
// and any other track that was default in the input gets the flag cleared.
func appendDisposition(dispositions []StreamDisposition, streamType string, outputIndex int, chosen, wasDefault bool) []StreamDisposition {
	spec := fmt.Sprintf("%s:%d", streamType, outputIndex)

	switch {
	case chosen:
		return append(dispositions, StreamDisposition{StreamSpec: spec, Value: "default"})
	case wasDefault:
		return append(dispositions, StreamDisposition{StreamSpec: spec, Value: "-default"})
	default:
		return dispositions
	}
}

// languageOf returns the stream's language, normalizing a missing language
// tag to "und" (undefined), which is how such tracks are usually tagged.
func languageOf(stream FFProbeStream) string {
	if stream.Tags.Language == "" {
		return "und"
	}

	return stream.Tags.Language
}

func matchesAnyLanguage(language string, languages []string) bool {
	for _, wanted := range languages {
		if strings.Contains(language, wanted) {
			return true
		}
	}

	return false
}

// isCommentary reports whether an audio stream looks like a commentary
// track, either via the comment disposition flag or its title.
func isCommentary(stream FFProbeStream) bool {
	return stream.Disposition.Comment == 1 ||
		strings.Contains(strings.ToLower(stream.Tags.Title), "commentary")
}

// splitLanguages splits a comma-separated language list, dropping empty
// entries.
func splitLanguages(languages string) []string {
	var result []string

	for _, language := range strings.Split(languages, ",") {
		language = strings.TrimSpace(language)
		if language != "" {
			result = append(result, language)
		}
	}

	return result
}

func containsInt(haystack []int, needle int) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}

	return false
}
