# Larcociel 4 Go

A Go/Ebitengine conversion of **DMA Intro 4**, an Atari ST intro by
**Larcociel of DMA**, using Demo Construction Kit **v1.0.13**. The original
screen combines two illustrated characters, sparkles and a large repeated
scrolltext produced through palette changes.

```sh
go run ./cmd/larcociel4
go run ./cmd/larcociel4 -mute
```

Space or Escape closes the intro. The 320 × 200 scene runs at 50 Hz. The font
contains twenty-seven six-by-seven color cells, with its original alphabet and
special space entry. DCK handles the repeated text layout and visible glyph
selection. Its palette grid stretches the thirty-five visible columns and
repeats the seven font rows in the original three-line bands and blank gaps.
The static bitmap mask preserves the characters and their individual colors.

The original bitmap, color font, scrolltext and YM6 chip soundtrack are
embedded. Palette updates upload a small cell bank; the rendering path performs
no GPU readback. Music is opened and played through DCK's audio facade.

```sh
go test ./...
go vet ./...
go run ./cmd/larcociel4 -capture captures/preview -frame 150 -frames 1
go run ./cmd/video
```

Video export creates a three-minute 50 fps H.264/AAC MP4, a PNG poster and a
JSON report in `recordings/`, with synchronized music. The website uses a
VP9/Opus WebM copy. Rendering is scaled by two to 640 × 400 pixels.

Original production: [Demozoo](https://demozoo.org/productions/79463/).

## Android

```sh
./scripts/run-android.sh --build-only
./scripts/run-android.sh
```

The build uses Java 17, Android SDK 36, NDK 28.2, Ebitengine 2.9.11 and the
included Gradle wrapper. It produces an ARM64 APK in
`android/app/build/outputs/apk/debug/app-debug.apk`. The host initializes Go
rendering and audio after the Android context is available, preserves landscape
orientation and keeps the screen awake. Android Back closes the activity.
The install command requires one authorized USB device; `ANDROID_SERIAL`
can select a specific device. Build products and machine settings stay local.
