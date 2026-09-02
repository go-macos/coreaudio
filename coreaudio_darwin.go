// Copyright (c) the go-macos authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package coreaudio

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/go-macos/objc"
)

const (
	coreAudio      = "/System/Library/Frameworks/CoreAudio.framework/CoreAudio"
	coreFoundation = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
)

// systemObject is the well-known id of the audio system itself, which is what
// answers "which devices are there" and "which one is the default".
const systemObject uint32 = 1

// The property selectors and scopes, spelled as CoreAudio's headers spell them.
var (
	propDevices       = fourCC("dev#") // kAudioHardwarePropertyDevices
	propDefaultOutput = fourCC("dOut") // kAudioHardwarePropertyDefaultOutputDevice
	propUID           = fourCC("uid ") // kAudioDevicePropertyDeviceUID
	propName          = fourCC("lnam") // kAudioObjectPropertyName
	propStreams       = fourCC("stm#") // kAudioDevicePropertyStreams

	scopeGlobal = fourCC("glob")
	scopeInput  = fourCC("inpt")
	scopeOutput = fourCC("outp")
)

// address is AudioObjectPropertyAddress: three 32-bit words, passed by pointer.
type address struct {
	selector, scope, element uint32
}

// at builds an address for the main element, which is the only element any
// property here has.
func at(selector, scope uint32) address {
	return address{selector: selector, scope: scope} // element 0 is kAudioObjectPropertyElementMain
}

// The seams. They are package vars so a test can drive every branch -- the
// framework that will not open, the property that will not answer -- on a
// machine where both work.
var (
	loadOnce sync.Once
	loadErr  error

	getPropertyData     func(obj uint32, addr *address, qualSize uint32, qual unsafe.Pointer, ioSize *uint32, out unsafe.Pointer) int32
	getPropertyDataSize func(obj uint32, addr *address, qualSize uint32, qual unsafe.Pointer, outSize *uint32) int32
	release             func(ref uintptr)

	caOpen = func(path string) (uintptr, error) {
		return purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	}
	caBind = func(ca, cf uintptr) {
		purego.RegisterLibFunc(&getPropertyData, ca, "AudioObjectGetPropertyData")
		purego.RegisterLibFunc(&getPropertyDataSize, ca, "AudioObjectGetPropertyDataSize")
		purego.RegisterLibFunc(&release, cf, "CFRelease")
	}
)

// load resolves the frameworks once.
func load() error {
	loadOnce.Do(func() {
		ca, err := caOpen(coreAudio)
		if err != nil {
			loadErr = fmt.Errorf("coreaudio: cannot open CoreAudio: %w", err)
			return
		}
		cf, err := caOpen(coreFoundation)
		if err != nil {
			loadErr = fmt.Errorf("coreaudio: cannot open CoreFoundation: %w", err)
			return
		}
		caBind(ca, cf)
	})
	return loadErr
}

// size asks how many bytes a property holds. A property that does not exist on
// an object answers with an error rather than a zero size, which is the
// difference between "this device has no output streams" and "this is not a
// device".
func size(obj uint32, a address) (uint32, error) {
	var n uint32
	if st := getPropertyDataSize(obj, &a, 0, nil, &n); st != 0 {
		return 0, statusError{op: "AudioObjectGetPropertyDataSize", status: st}
	}
	return n, nil
}

// data reads a property into out, which must have room for n bytes.
func data(obj uint32, a address, out unsafe.Pointer, n uint32) error {
	if st := getPropertyData(obj, &a, 0, nil, &n, out); st != 0 {
		return statusError{op: "AudioObjectGetPropertyData", status: st}
	}
	return nil
}

// text reads a CFString property and copies it into a Go string.
//
// The CFStringRef comes back RETAINED -- a "get" that gives ownership, which is
// unusual enough that missing it leaks one string per device per call -- so it
// is released here. A device with no such property answers with a null
// reference rather than an error, and an empty name is the honest rendering of
// that.
func text(obj uint32, a address) (string, error) {
	var ref uintptr
	if err := data(obj, a, unsafe.Pointer(&ref), uint32(unsafe.Sizeof(ref))); err != nil {
		return "", err
	}
	if ref == 0 {
		return "", nil
	}
	defer release(ref)
	return objc.GoString(objc.ID(ref)), nil
}

// streams counts the streams a device publishes in one direction. The count is
// the property's SIZE divided by the width of an id: the ids themselves say
// nothing this package needs, and asking for them would allocate a slice per
// device to throw away.
func streams(dev uint32, scope uint32) int {
	n, err := size(dev, at(propStreams, scope))
	if err != nil {
		// A device that will not say has none as far as anyone can tell, and
		// saying so beats failing the whole enumeration over one device.
		return 0
	}
	return int(n) / 4
}

// describe reads one device.
func describe(id uint32) (Device, error) {
	uid, err := text(id, at(propUID, scopeGlobal))
	if err != nil {
		return Device{}, fmt.Errorf("coreaudio: device %d has no unique id: %w", id, err)
	}
	name, err := text(id, at(propName, scopeGlobal))
	if err != nil {
		// A device with no name is usable -- the UID is what playback needs --
		// so this is not fatal, and the name is left empty.
		name = ""
	}
	return Device{
		UID:     uid,
		Name:    name,
		Outputs: streams(id, scopeOutput),
		Inputs:  streams(id, scopeInput),
	}, nil
}

// Devices lists every audio device the system knows about, playback and capture
// alike, in the order CoreAudio reports them.
//
// Capture devices are included deliberately. Filtering them out here would mean
// a caller looking for a device by name could not tell "no such device" from "a
// device by that name that cannot play", and those call for different words.
func Devices() ([]Device, error) {
	if err := load(); err != nil {
		return nil, err
	}
	a := at(propDevices, scopeGlobal)
	n, err := size(systemObject, a)
	if err != nil {
		return nil, fmt.Errorf("coreaudio: cannot count the devices: %w", err)
	}
	count := int(n) / 4
	if count == 0 {
		return nil, nil
	}
	ids := make([]uint32, count)
	if err := data(systemObject, a, unsafe.Pointer(&ids[0]), n); err != nil {
		return nil, fmt.Errorf("coreaudio: cannot list the devices: %w", err)
	}
	out := make([]Device, 0, count)
	for _, id := range ids {
		d, err := describe(id)
		if err != nil {
			// One unreadable device must not hide the rest: an enumeration that
			// fails wholesale on a single odd device is how a caller ends up
			// believing the machine has no audio at all.
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// DefaultOutput is the device the system is currently sending sound to.
//
// It is what a program gets by saying nothing, and knowing which one it is lets
// a program say -- truthfully -- where the sound went.
func DefaultOutput() (Device, error) {
	if err := load(); err != nil {
		return Device{}, err
	}
	var id uint32
	if err := data(systemObject, at(propDefaultOutput, scopeGlobal),
		unsafe.Pointer(&id), uint32(unsafe.Sizeof(id))); err != nil {
		return Device{}, fmt.Errorf("coreaudio: cannot read the default output: %w", err)
	}
	d, err := describe(id)
	if err != nil {
		return Device{}, err
	}
	return d, nil
}

// statusError is an OSStatus that came back from CoreAudio.
type statusError struct {
	op     string
	status int32
}

// Error renders the status as its four-character code where it is one, because
// that is how CoreAudio's own documentation names these, and as a number where
// it is not.
func (e statusError) Error() string {
	return fmt.Sprintf("%s: %s", e.op, statusText(e.status))
}
