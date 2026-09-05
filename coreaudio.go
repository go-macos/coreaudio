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

import (
	"errors"
	"fmt"
)

// Device is one audio device the system knows about.
type Device struct {
	// UID identifies the device to the rest of CoreAudio, and keeps identifying
	// it after a reboot or a re-plug. It is what a setting should store; the
	// name is what a person should read.
	UID string
	// Name is the name shown in Sound settings.
	Name string
	// Outputs is how many output streams the device publishes, and
	// Inputs is how many input ones. A device is not necessarily either: a
	// microphone has no outputs, and asking it to play sound would fail for a
	// reason nobody would guess from "device not found".
	Outputs, Inputs int

	// id is the AudioObjectID this device answers to.
	//
	// ⛔ UNEXPORTED, AND IT IS NOT AN IDENTITY. CoreAudio hands these out per
	// boot and reuses them: a device unplugged and plugged back in may get a
	// different one, and the one it had may belong to something else. So it is
	// good for talking to a device you have JUST listed and for nothing else,
	// which is exactly what [Device.Muted] and [Device.SetMuted] do -- while
	// UID is what a setting stores.
	id uint32
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

// ErrNoMute says the device publishes no mute switch for capture.
//
// ⚠ IT IS COMMON, not exceptional. A mute switch is a property a device MAY
// have: many capture devices publish none and are silenced by setting their
// volume to zero instead, and some publish one per channel rather than a master
// one. So a caller that offers "mute the microphone" has to be ready to say it
// cannot, in words, rather than to fail.
var ErrNoMute = errors.New("coreaudio: this device has no capture mute switch")

// CanMute reports whether the device has a capture mute switch to work.
//
// ⛔ ASK BEFORE OFFERING. A menu row or a key that silently does nothing is
// worse than one that is not there: it gets pressed again.
func (d Device) CanMute() bool { return platformCanMute(d) }

// Muted reads the device's capture mute switch.
//
// ⛔ READ BEFORE WRITING, always. A key that means "mute" is pressed blind, and
// the only thing that knows whether the microphone is already off is the
// device -- a person may have used the switch on the hardware, or another
// application, since anything here last looked.
func (d Device) Muted() (bool, error) { return platformMuted(d) }

// SetMuted turns the device's capture mute switch on or off.
func (d Device) SetMuted(mute bool) error { return platformSetMuted(d, mute) }

// ErrNoVolume says the device publishes no capture level.
var ErrNoVolume = errors.New("coreaudio: this device has no capture level")

// CanSetVolume reports whether the device has a capture level to move.
//
// ⭐ IT IS THE OTHER WAY TO SILENCE A MICROPHONE, and on some devices the only
// one: measured on this machine, the MacBook's own microphone has a mute switch
// and the VITURE headset's has none. A caller that offers "mute the microphone"
// should ask for [Device.CanMute] first and fall back to setting this to zero.
func (d Device) CanSetVolume() bool { return platformCanSetVolume(d) }

// Volume reads the device's capture level, 0 to 1.
func (d Device) Volume() (float32, error) { return platformVolume(d) }

// SetVolume writes it, clamped to 0..1.
func (d Device) SetVolume(v float32) error { return platformSetVolume(d, v) }
