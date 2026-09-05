// Copyright (c) the go-macos authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package coreaudio

import (
	"errors"
	"testing"
)

// TestEveryCallRefusesOffMacOS, and refuses in a way a caller can recognise.
//
// The alternative -- an empty device list and no error -- is worse than a
// refusal: it is indistinguishable from a machine with no sound card, and a
// caller would go on to report that the glasses have no audio output when what
// happened is that this is not a Mac.
func TestEveryCallRefusesOffMacOS(t *testing.T) {
	devs, err := Devices()
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("Devices() = %v, want ErrUnsupported", err)
	}
	if devs != nil {
		t.Errorf("Devices() also returned %v, which invites a caller to use it", devs)
	}
	d, err := DefaultOutput()
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("DefaultOutput() = %v, want ErrUnsupported", err)
	}
	if d != (Device{}) {
		t.Errorf("DefaultOutput() also returned %v", d)
	}
}

// TestSilencingRefusesOffMacOSToo, and CanMute answers NO rather than failing.
//
// ⛔ THE TWO ANSWER DIFFERENTLY ON PURPOSE. CanMute is a question a caller asks
// BEFORE offering a row or a key, and "false" is the honest answer everywhere
// off macOS -- an error there would make every caller write the same two lines
// to turn it back into a false. Reading or writing is a caller that has decided
// to act, and that one has to be told why nothing happened.
func TestSilencingRefusesOffMacOSToo(t *testing.T) {
	var d Device
	if d.CanMute() || d.CanSetVolume() {
		t.Error("a device off macOS claims a control")
	}
	if _, err := d.Muted(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Muted() = %v, want ErrUnsupported", err)
	}
	if err := d.SetMuted(true); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetMuted() = %v, want ErrUnsupported", err)
	}
	if _, err := d.Volume(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Volume() = %v, want ErrUnsupported", err)
	}
	if err := d.SetVolume(0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetVolume() = %v, want ErrUnsupported", err)
	}
}
