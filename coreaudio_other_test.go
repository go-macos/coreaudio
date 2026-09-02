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
