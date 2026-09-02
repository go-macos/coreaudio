# coreaudio

Names the machine's audio devices, in pure Go with `CGO_ENABLED=0`, so a program
can send its sound somewhere in particular instead of wherever the system
happens to be pointing.

```go
devs, err := coreaudio.Devices()
for _, d := range devs {
    fmt.Println(d) // "VITURE" [1 out] 0ED43331-0000-0000-2021-010380100A78
}

out, err := coreaudio.DefaultOutput()
fmt.Println("sound is going to", out.Name)
```

The need is ordinary and the default is wrong often enough to matter. A film
played on a pair of XR glasses draws its picture on the glasses and, with no
device named, plays its sound out of the Mac's own speakers. Nothing reports an
error, because nothing went wrong: the sound went to the default output, which
is not where the person is looking.

`Device.UID` is the string AVFoundation and AudioQueue both take to be told
where to play, and it keeps identifying the device after a reboot or a re-plug,
unlike the numeric ids beside it, which do not.

Capture devices are listed too. Filtering them out here would leave a caller
looking for a device by name unable to tell *no such device* from *a device by
that name that cannot play*, and those call for different words.

Everything is bound through [purego](https://github.com/ebitengine/purego):
no cgo, no Xcode, and the package cross-compiles.
