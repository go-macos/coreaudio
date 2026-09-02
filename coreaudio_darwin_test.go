// Copyright (c) the go-macos authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package coreaudio

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// resetLoad puts the lazily-resolved frameworks back for the next test.
func resetLoad(t *testing.T) {
	t.Helper()
	open, bind := caOpen, caBind
	t.Cleanup(func() {
		caOpen, caBind = open, bind
		loadOnce, loadErr = sync.Once{}, nil
		getPropertyData, getPropertyDataSize, release = nil, nil, nil
	})
	loadOnce, loadErr = sync.Once{}, nil
	getPropertyData, getPropertyDataSize, release = nil, nil, nil
}

// TestDevicesFindsTheMachinesOwnAudio is the test that would notice the
// binding being wrong in any of the ways a binding is wrong: the wrong
// selector, the wrong struct layout, a CFString read as something else. A Mac
// always has at least one output.
func TestDevicesFindsTheMachinesOwnAudio(t *testing.T) {
	resetLoad(t)
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) == 0 {
		t.Fatal("no audio devices at all, on a Mac")
	}
	playable := 0
	for _, d := range devs {
		if d.UID == "" {
			t.Errorf("%v has no unique id, so nothing could be told to play there", d)
		}
		if d.Outputs < 0 || d.Inputs < 0 {
			t.Errorf("%v has a negative number of streams", d)
		}
		if d.CanPlay() {
			playable++
		}
	}
	if playable == 0 {
		t.Error("not one device can play sound, on a Mac")
	}
	t.Logf("%d device(s):", len(devs))
	for _, d := range devs {
		t.Logf("  %v", d)
	}
}

// TestDefaultOutputIsOneOfTheDevices, and can play. A default output that is
// not in the list, or cannot play, would mean the two properties are being read
// off different objects.
func TestDefaultOutputIsOneOfTheDevices(t *testing.T) {
	resetLoad(t)
	def, err := DefaultOutput()
	if err != nil {
		t.Fatalf("DefaultOutput: %v", err)
	}
	if !def.CanPlay() {
		t.Errorf("the default output %v cannot play", def)
	}
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	for _, d := range devs {
		if d.UID == def.UID {
			t.Logf("sound is going to %v", def)
			return
		}
	}
	t.Errorf("the default output %v is not among the %d devices listed", def, len(devs))
}

// TestNamesAreReadAsStrings guards the CFString path specifically: a reference
// read as an integer, or released twice, does not fail politely.
func TestNamesAreReadAsStrings(t *testing.T) {
	resetLoad(t)
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	named := 0
	for _, d := range devs {
		if d.Name != "" {
			named++
		}
		if strings.ContainsRune(d.Name, 0) {
			t.Errorf("the name of %v carries a NUL, so it was not read as a string", d)
		}
		if strings.ContainsRune(d.UID, 0) {
			t.Errorf("the unique id of %v carries a NUL", d)
		}
	}
	if named == 0 {
		t.Error("not one device has a name, so the name property is not being read")
	}
	// Called twice, the same names come back. A reference released one time too
	// many would have been reused by then.
	again, err := Devices()
	if err != nil {
		t.Fatalf("Devices, second time: %v", err)
	}
	if len(again) != len(devs) {
		t.Fatalf("%d devices, then %d", len(devs), len(again))
	}
	for i := range devs {
		if devs[i] != again[i] {
			t.Errorf("device %d read as %v, then as %v", i, devs[i], again[i])
		}
	}
}

func TestAFrameworkThatWillNotOpenIsReportedOnce(t *testing.T) {
	resetLoad(t)
	calls := 0
	caOpen = func(string) (uintptr, error) { calls++; return 0, errors.New("gone") }

	if _, err := Devices(); err == nil {
		t.Error("Devices succeeded with no CoreAudio")
	}
	if _, err := DefaultOutput(); err == nil {
		t.Error("DefaultOutput succeeded with no CoreAudio")
	}
	if calls != 1 {
		t.Errorf("the framework was looked for %d times, want 1", calls)
	}
}

// TestTheSecondFrameworkIsReportedToo: CoreFoundation is opened separately, and
// a failure there must not be reported as a CoreAudio failure.
func TestTheSecondFrameworkIsReportedToo(t *testing.T) {
	resetLoad(t)
	caOpen = func(path string) (uintptr, error) {
		if strings.Contains(path, "CoreFoundation") {
			return 0, errors.New("gone")
		}
		return 1, nil
	}
	_, err := Devices()
	if err == nil {
		t.Fatal("Devices succeeded with no CoreFoundation")
	}
	if !strings.Contains(err.Error(), "CoreFoundation") {
		t.Errorf("the error says %q, which does not name the framework that failed", err)
	}
}

// fake binds the two property calls to functions a test drives.
func fake(t *testing.T, sizeFn func(uint32, *address, *uint32) int32, dataFn func(uint32, *address, *uint32, unsafe.Pointer) int32) {
	t.Helper()
	resetLoad(t)
	caOpen = func(string) (uintptr, error) { return 1, nil }
	caBind = func(uintptr, uintptr) {
		getPropertyDataSize = func(obj uint32, a *address, _ uint32, _ unsafe.Pointer, n *uint32) int32 {
			return sizeFn(obj, a, n)
		}
		getPropertyData = func(obj uint32, a *address, _ uint32, _ unsafe.Pointer, n *uint32, out unsafe.Pointer) int32 {
			return dataFn(obj, a, n, out)
		}
		release = func(uintptr) {}
	}
}

func TestASystemThatWillNotCountItsDevices(t *testing.T) {
	fake(t, func(uint32, *address, *uint32) int32 { return 0x216f626a }, // '!obj'
		func(uint32, *address, *uint32, unsafe.Pointer) int32 { return 0 })
	_, err := Devices()
	if err == nil {
		t.Fatal("Devices succeeded when the count failed")
	}
	if !strings.Contains(err.Error(), "!obj") {
		t.Errorf("the error says %q, which does not name the status", err)
	}
}

func TestASystemWithNoDevicesAtAll(t *testing.T) {
	fake(t, func(_ uint32, _ *address, n *uint32) int32 { *n = 0; return 0 },
		func(uint32, *address, *uint32, unsafe.Pointer) int32 { return 0 })
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 0 {
		t.Errorf("%d devices came back from a system with none", len(devs))
	}
}

func TestASystemThatCountsItsDevicesAndWillNotListThem(t *testing.T) {
	fake(t, func(_ uint32, _ *address, n *uint32) int32 { *n = 8; return 0 },
		func(_ uint32, a *address, _ *uint32, _ unsafe.Pointer) int32 {
			if a.selector == propDevices {
				return -1
			}
			return 0
		})
	if _, err := Devices(); err == nil {
		t.Fatal("Devices succeeded when the listing failed")
	}
}

// TestOneUnreadableDeviceDoesNotHideTheRest: an enumeration that fails
// wholesale on a single odd device is how a caller comes to believe a machine
// has no audio.
func TestOneUnreadableDeviceDoesNotHideTheRest(t *testing.T) {
	fake(t, func(_ uint32, a *address, n *uint32) int32 {
		switch a.selector {
		case propDevices:
			*n = 8 // two devices
		case propStreams:
			*n = 4 // one stream each
		}
		return 0
	}, func(obj uint32, a *address, _ *uint32, out unsafe.Pointer) int32 {
		switch {
		case a.selector == propDevices:
			ids := (*[2]uint32)(out)
			ids[0], ids[1] = 11, 22
			return 0
		case obj == 11 && a.selector == propUID:
			return -1 // this one will not say who it is
		default:
			*(*uintptr)(out) = 0 // a null CFString: no name, no error
			return 0
		}
	})
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("%d devices survived one unreadable device, want 1", len(devs))
	}
	if devs[0].Outputs != 1 || devs[0].Inputs != 1 {
		t.Errorf("the surviving device reports %d out and %d in, want 1 each", devs[0].Outputs, devs[0].Inputs)
	}
}

func TestADefaultOutputThatWillNotAnswer(t *testing.T) {
	fake(t, func(_ uint32, _ *address, n *uint32) int32 { *n = 4; return 0 },
		func(_ uint32, a *address, _ *uint32, _ unsafe.Pointer) int32 {
			if a.selector == propDefaultOutput {
				return -1
			}
			return 0
		})
	if _, err := DefaultOutput(); err == nil {
		t.Fatal("DefaultOutput succeeded when the property failed")
	}
}

// TestADefaultOutputThatCannotBeDescribed is the other half: the id came back
// and the device behind it did not.
func TestADefaultOutputThatCannotBeDescribed(t *testing.T) {
	fake(t, func(_ uint32, _ *address, n *uint32) int32 { *n = 4; return 0 },
		func(_ uint32, a *address, _ *uint32, out unsafe.Pointer) int32 {
			switch a.selector {
			case propDefaultOutput:
				*(*uint32)(out) = 99
				return 0
			case propUID:
				return -1
			}
			return 0
		})
	if _, err := DefaultOutput(); err == nil {
		t.Fatal("DefaultOutput succeeded describing a device that would not answer")
	}
}

// TestADeviceThatWillNotCountItsStreams has none as far as anyone can tell,
// rather than failing the enumeration.
func TestADeviceThatWillNotCountItsStreams(t *testing.T) {
	fake(t, func(_ uint32, a *address, n *uint32) int32 {
		if a.selector == propStreams {
			return -1
		}
		*n = 4
		return 0
	}, func(_ uint32, a *address, _ *uint32, out unsafe.Pointer) int32 {
		if a.selector == propDevices {
			*(*uint32)(out) = 7
		} else {
			*(*uintptr)(out) = 0
		}
		return 0
	})
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 1 || devs[0].Outputs != 0 || devs[0].Inputs != 0 {
		t.Errorf("got %v, want one device with no streams", devs)
	}
}

// TestADeviceThatWillNotSayItsNameIsStillUsable: playback needs the unique id,
// not the name, so a nameless device is listed rather than dropped.
func TestADeviceThatWillNotSayItsNameIsStillUsable(t *testing.T) {
	fake(t, func(_ uint32, a *address, n *uint32) int32 {
		if a.selector == propStreams {
			*n = 8
			return 0
		}
		*n = 4
		return 0
	}, func(_ uint32, a *address, _ *uint32, out unsafe.Pointer) int32 {
		switch a.selector {
		case propDevices:
			*(*uint32)(out) = 3
			return 0
		case propName:
			return -1 // it will not say
		default:
			*(*uintptr)(out) = 0
			return 0
		}
	})
	devs, err := Devices()
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("%d devices, want the nameless one to survive", len(devs))
	}
	if devs[0].Name != "" {
		t.Errorf("the name came back as %q, want it left empty", devs[0].Name)
	}
	if !devs[0].CanPlay() {
		t.Error("the nameless device cannot play, so it was not read at all")
	}
}
