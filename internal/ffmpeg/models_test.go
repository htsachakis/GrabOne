package ffmpeg

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "release build",
			input: "ffmpeg version 8.0 Copyright (c) 2000-2026 the FFmpeg developers\nbuilt with gcc 14",
			want:  "8.0",
		},
		{
			name:  "git build",
			input: "ffmpeg version N-125875-g5d4d3bdc61-20260731 Copyright (c) 2000-2026 the FFmpeg developers",
			want:  "N-125875-g5d4d3bdc61-20260731",
		},
		{
			name:  "distribution build",
			input: "ffprobe version 6.1.1-3ubuntu5 Copyright (c) 2007-2023 the FFmpeg developers",
			want:  "6.1.1-3ubuntu5",
		},
		{
			name:  "unexpected output keeps the first line",
			input: "something else entirely\nsecond line",
			want:  "something else entirely",
		},
		{name: "empty", input: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseVersion(test.input); got != test.want {
				t.Errorf("ParseVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestProberAvailability(t *testing.T) {
	if (&Prober{}).Available() {
		t.Error("a prober with no executable is not available")
	}
	if !NewProber("ffprobe").Available() {
		t.Error("a prober with an executable is available")
	}

	var missing *Prober
	if missing.Available() {
		t.Error("a nil prober is not available")
	}
}

func TestInspectWithoutFFprobe(t *testing.T) {
	// Inspection is optional: without FFprobe it fails cleanly rather than
	// panicking, and the caller treats that as "details unavailable".
	if _, err := (&Prober{}).Inspect(t.Context(), "clip.mkv"); err == nil {
		t.Error("inspecting without ffprobe should report an error")
	}
}
