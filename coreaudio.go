// Copyright (c) the go-macos authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Package coreaudio names the machine's audio devices, so a program can send
// its sound somewhere in particular instead of wherever the system happens to
// be pointing.
//
// The need is ordinary and the default is wrong often enough to matter: a film
// played on a pair of XR glasses draws its picture on the glasses and, with no
// device named, plays its sound out of the Mac's own speakers. Nothing reports
// an error, because nothing went wrong -- the sound simply went to the default
// output, which is not where the person is looking.
//
// What is here is enumeration and identity, not playback. A device's [Device.UID]
// is the string AVFoundation and AudioQueue both take to be told where to play,
// and it is stable across reboots and re-plugs, unlike the numeric ids beside
// it, which are not.
package coreaudio

import "fmt"

// Device is one audio device the system knows about.
type Device struct {
	// UID identifies the device to the rest of CoreAudio, and keeps identifying
	// it after a reboot or a re-plug. It is what a setting should store; the
	// name is what a person should read.
	UID string
	// Name is the name shown in Sound settings.
	Name string
	// Outputs is how many output streams the device publishes, and Inputs how
	// many input ones. A device is not necessarily either: a microphone has no
	// outputs, and asking it to play sound would fail for a reason nobody would
	// guess from "device not found".
	Outputs, Inputs int
}

// CanPlay reports whether the device has anywhere to put sound.
func (d Device) CanPlay() bool { return d.Outputs > 0 }

// String renders the device the way a listing reads.
func (d Device) String() string {
	name := d.Name
	if name == "" {
		name = "(unnamed)"
	}
	kind := "silent"
	switch {
	case d.Outputs > 0 && d.Inputs > 0:
		kind = fmt.Sprintf("%d out, %d in", d.Outputs, d.Inputs)
	case d.Outputs > 0:
		kind = fmt.Sprintf("%d out", d.Outputs)
	case d.Inputs > 0:
		kind = fmt.Sprintf("%d in", d.Inputs)
	}
	return fmt.Sprintf("%q [%s] %s", name, kind, d.UID)
}

// fourCC packs a four-character code the way CoreAudio's selectors are written
// in its headers, so the constants below can be read as the documentation
// writes them instead of as hexadecimal nobody can check.
//
// It panics on anything but four bytes. Every caller is a constant in this
// package, so a wrong length is a typo in a literal and not a runtime
// condition: failing at the first call beats returning a selector that names
// nothing and reports "unknown property" from somewhere else entirely.
func fourCC(s string) uint32 {
	if len(s) != 4 {
		panic("coreaudio: a four-character code must be four bytes, got " + fmt.Sprint(len(s)))
	}
	return uint32(s[0])<<24 | uint32(s[1])<<16 | uint32(s[2])<<8 | uint32(s[3])
}

// statusText renders an OSStatus the way CoreAudio's headers name these: as the
// four-character code it usually is, and as a plain number when its bytes are
// not printable, which some of them are not.
//
// The distinction earns its keep when a call fails. "OSStatus 560947818" sends
// a reader to a search engine; "'!obj'" is findable in a header and means the
// object id was not a device at all.
func statusText(st int32) string {
	b := [4]byte{byte(st >> 24), byte(st >> 16), byte(st >> 8), byte(st)}
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return fmt.Sprintf("OSStatus %d", st)
		}
	}
	return fmt.Sprintf("%q (OSStatus %d)", b, st)
}
