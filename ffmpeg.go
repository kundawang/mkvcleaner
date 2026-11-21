package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/alessio/shellescape.v1"
)

// StreamDisposition describes a disposition adjustment for an output
// stream, e.g. marking the default audio track.
type StreamDisposition struct {
	StreamSpec string // output stream specifier, e.g. "a:0" or "s:1"
	Value      string // disposition value, e.g. "default" or "-default"
}

type FFMpeg struct {
	inputFilePath  string
	outputFilePath string
	audioTracks    []int
	subtitleTracks []int
	videoTracks    []int

	// mapOrder, when non-nil, lists the input streams to map in the exact
	// order they should appear in the output (used by the per-track-type
	// strategy). When nil, tracks are mapped grouped by type (video,
	// audio, subtitles) as before.
	mapOrder []int

	// dispositions holds output stream disposition adjustments, e.g.
	// marking the default audio/subtitle track.
	dispositions []StreamDisposition
}

func (ffmpeg *FFMpeg) FormatCommandParts() []string {
	// Extract the directory and file name from the path
	dir, file := filepath.Split(ffmpeg.inputFilePath)

	// Set output path
	ffmpeg.outputFilePath = filepath.Join(dir, ".tmp."+file)

	// Escape paths
	escapedInputFilePath := shellescape.Quote(ffmpeg.inputFilePath)
	escapedOutputFilePath := shellescape.Quote(dir + ".tmp." + file)

	trackArgs := []string{}

	if ffmpeg.mapOrder != nil {
		// Strategy mode: map tracks in the given (original stream) order.
		for _, track := range ffmpeg.mapOrder {
			trackArgs = append(trackArgs, fmt.Sprintf("-map 0:%d", track))
		}
	} else {
		// Legacy mode: map tracks grouped by type (video, audio,
		// subtitles).
		for _, track := range ffmpeg.videoTracks {
			trackArgs = append(trackArgs, fmt.Sprintf("-map 0:%d", track))
		}
		for _, track := range ffmpeg.audioTracks {
			trackArgs = append(trackArgs, fmt.Sprintf("-map 0:%d", track))
		}
		for _, track := range ffmpeg.subtitleTracks {
			trackArgs = append(trackArgs, fmt.Sprintf("-map 0:%d", track))
		}
	}

	// Go through the disposition adjustments to append to ffmpeg command
	for _, disposition := range ffmpeg.dispositions {
		trackArgs = append(trackArgs, fmt.Sprintf("-disposition:%s %s", disposition.StreamSpec, disposition.Value))
	}

	// Build and return the ffmpeg command as a slice of strings
	return append(
		[]string{
			"ffmpeg",
			"-i",
			escapedInputFilePath,
			"-c",
			"copy",
		},
		append(trackArgs, escapedOutputFilePath)...,
	)
}

func (ffmpeg *FFMpeg) Run() error {
	// Get the formatted command parts
	cmdParts := ffmpeg.FormatCommandParts()

	// Run the ffmpeg command
	cmd := exec.Command("sh", "-c", strings.Join(cmdParts, " "))

	return cmd.Run()
}
