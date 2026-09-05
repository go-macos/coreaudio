// Copyright (c) the go-macos authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package coreaudio

import "errors"

// ErrUnsupported is returned by every call off macOS. CoreAudio is a macOS
// framework; there is nothing here to fall back to, and pretending otherwise
// would hand a caller an empty device list that looks like a machine with no
// sound card.
var ErrUnsupported = errors.New("coreaudio: only macOS has CoreAudio")

// Devices reports [ErrUnsupported].
func Devices() ([]Device, error) { return nil, ErrUnsupported }

// DefaultOutput reports [ErrUnsupported].
func DefaultOutput() (Device, error) { return Device{}, ErrUnsupported }

// platformCanMute reports no, because there is no device to ask.
func platformCanMute(Device) bool { return false }

// platformMuted reports [ErrUnsupported].
func platformMuted(Device) (bool, error) { return false, ErrUnsupported }

// platformSetMuted reports [ErrUnsupported].
func platformSetMuted(Device, bool) error { return ErrUnsupported }

// platformCanSetVolume reports no, because there is no device to ask.
func platformCanSetVolume(Device) bool { return false }

// platformVolume reports [ErrUnsupported].
func platformVolume(Device) (float32, error) { return 0, ErrUnsupported }

// platformSetVolume reports [ErrUnsupported].
func platformSetVolume(Device, float32) error { return ErrUnsupported }
