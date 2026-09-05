// Copyright (c) the go-macos authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package coreaudio

import (
	"errors"
	"strings"
	"testing"
)

func TestCanPlayNeedsAnOutput(t *testing.T) {
	cases := []struct {
		name string
		d    Device
		want bool
	}{
		{"a pair of glasses", Device{Outputs: 2}, true},
		{"a microphone", Device{Inputs: 1}, false},
		{"an interface that does both", Device{Outputs: 2, Inputs: 2}, true},
		{"a device that does neither", Device{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.d.CanPlay(); got != c.want {
				t.Errorf("CanPlay() = %v for %v", got, c.d)
			}
		})
	}
}

func TestDeviceStringSaysWhatItCanDo(t *testing.T) {
	cases := []struct {
		name string
		d    Device
		want []string
	}{
		{"a playback device", Device{UID: "AppleHDA:0", Name: "VITURE", Outputs: 2}, []string{`"VITURE"`, "2 out", "AppleHDA:0"}},
		{"a capture device", Device{UID: "mic", Name: "VITURE Microphone", Inputs: 1}, []string{"1 in"}},
		{"a device that does both", Device{UID: "iface", Name: "Scarlett", Outputs: 4, Inputs: 2}, []string{"4 out, 2 in"}},
		{"a device that does neither", Device{UID: "odd", Name: "Aggregate"}, []string{"silent"}},
		{"a device with no name", Device{UID: "nameless", Outputs: 2}, []string{"(unnamed)"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.d.String()
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("%q does not mention %q", got, want)
				}
			}
		})
	}
}

func TestFourCCReadsAsTheHeadersWriteIt(t *testing.T) {
	// The values CoreAudio's own headers give for these selectors. Checking
	// them against literals is the only way to know the packing is the right
	// way round: a reversed code is a valid number that names nothing, and the
	// failure surfaces as "unknown property" much later.
	cases := []struct {
		code string
		want uint32
	}{
		{"dev#", 0x64657623},
		{"glob", 0x676c6f62},
		{"outp", 0x6f757470},
		{"uid ", 0x75696420},
	}
	for _, c := range cases {
		if got := fourCC(c.code); got != c.want {
			t.Errorf("fourCC(%q) = %#08x, want %#08x", c.code, got, c.want)
		}
	}
}

func TestFourCCRefusesAnythingButFourBytes(t *testing.T) {
	for _, s := range []string{"", "abc", "abcde"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("fourCC(%q) was accepted", s)
				}
			}()
			_ = fourCC(s)
		}()
	}
	// The positive control: four bytes do not panic, so the panics above are
	// about the length and not about the function.
	if fourCC("dev#") == 0 {
		t.Error("fourCC refused a well-formed code")
	}
}

func TestStatusTextNamesWhatCanBeNamed(t *testing.T) {
	// '!obj' -- kAudioHardwareBadObjectError -- is what asking a non-device for
	// a device property returns, and it is findable in a header under that
	// spelling and not under its decimal value.
	const badObject = int32(0x216f626a)
	if got := statusText(badObject); !strings.Contains(got, "!obj") {
		t.Errorf("statusText(%#x) = %q, want it to name the code", badObject, got)
	}
	// A status whose bytes are not printable must not be rendered as mojibake
	// dressed up as a code.
	if got := statusText(-50); strings.Contains(got, `"`) {
		t.Errorf("statusText(-50) = %q, want a plain number", got)
	}
	if got := statusText(-50); !strings.Contains(got, "-50") {
		t.Errorf("statusText(-50) = %q, want it to give the number", got)
	}
}

// TestTheTwoWaysToSilenceAreSeparateQuestions.
//
// ⭐ MEASURED, AND THE ANSWER IS NOT THE SAME FOR EVERY DEVICE. On the machine
// this was written for: the MacBook's own microphone publishes BOTH a mute
// switch and a capture level; the VITURE headset's microphone publishes
// NEITHER, while every other input device on the machine publishes at least
// one. So a caller offering "mute the microphone" has to ask, and has to be
// able to say it cannot -- which is why there are two errors and two questions
// rather than one boolean and a hope.
func TestTheTwoWaysToSilenceAreSeparateQuestions(t *testing.T) {
	if !errors.Is(ErrNoMute, ErrNoMute) || errors.Is(ErrNoMute, ErrNoVolume) {
		t.Error("the two refusals cannot be told apart")
	}
	for _, e := range []error{ErrNoMute, ErrNoVolume} {
		if !strings.HasPrefix(e.Error(), "coreaudio: ") {
			t.Errorf("%v does not say which package refused", e)
		}
	}
	// A device with no inputs has neither, whatever the platform: asking a
	// loudspeaker to mute its microphone is a question about nothing.
	out := Device{Name: "some speakers", Outputs: 2}
	if out.CanMute() || out.CanSetVolume() {
		t.Error("a device with no input claims a capture control")
	}
}
