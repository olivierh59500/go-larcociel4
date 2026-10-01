// Package demo composes DMA Intro 4's hardware palette scroller through DCK.
package demo

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/font"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sound"
	playback "github.com/olivierh59500/democonstructionkit/sound/ebiten"
	"github.com/olivierh59500/go-larcociel4/assets"
)

const Width, Height, FPS = 320, 200, 50

type Game struct {
	images                         []*ebiten.Image
	background, mask, atlas, stage *ebiten.Image
	lookup                         *composite.IndexedPalette
	scroll                         *scrolling.Scrolling
	window                         *scrolling.CyclicWindow
	grid                           *composite.PaletteGrid
	colors, transparent            [35 * 8]color.NRGBA
	fontPixels                     []byte
	message                        []byte
	player                         *playback.Player
	visual                         *sound.Stream
	pcm                            [960 * 8]byte
	tick                           int
	closed                         bool
}

func resource(name string) ([]byte, error) { return assets.Files.ReadFile("original/" + name) }
func NewGame(mute bool) (_ *Game, err error) {
	g := &Game{}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	load := func(name string) (*ebiten.Image, image.Image, error) {
		b, e := resource(name)
		if e != nil {
			return nil, nil, e
		}
		im, _, e := image.Decode(bytes.NewReader(b))
		if e != nil {
			return nil, nil, e
		}
		texture := ebiten.NewImageFromImage(im)
		g.images = append(g.images, texture)
		return texture, im, nil
	}
	g.background, _, err = load("background.png")
	if err != nil {
		return nil, err
	}
	g.mask, _, err = load("palette-mask.png")
	if err != nil {
		return nil, err
	}
	var original image.Image
	g.atlas, original, err = load("font.png")
	if err != nil {
		return nil, err
	}
	g.fontPixels = make([]byte, 27*6*7*4)
	for y := 0; y < 7; y++ {
		for x := 0; x < 27*6; x++ {
			r, green, b, a := original.At(x, y).RGBA()
			at := (y*27*6 + x) * 4
			g.fontPixels[at] = byte(r >> 8)
			g.fontPixels[at+1] = byte(green >> 8)
			g.fontPixels[at+2] = byte(b >> 8)
			g.fontPixels[at+3] = byte(a >> 8)
		}
	}
	g.message, err = resource("message.txt")
	if err != nil {
		return nil, err
	}
	metrics, err := font.NewGrid(font.Grid{Bounds: g.atlas.Bounds(), Cell: image.Pt(6, 7), Columns: 27, Order: "@ABCDEFGHIJKLMNOPQRSTUVWXYZ", Advance: 6, SpaceAdvance: 6})
	if err != nil {
		return nil, err
	}
	g.scroll, err = scrolling.New(scrolling.Config{Text: string(g.message), Font: "native", Fonts: map[string]scrolling.Face{"native": {Atlas: g.atlas, Metrics: metrics}}, Speed: 50, X: 41, Y: 0, Repeat: true, Gap: 6})
	if err != nil {
		return nil, err
	}
	g.window, err = scrolling.NewCyclicWindow(g.scroll, scrolling.CyclicWindowConfig{Scale: 1, Minimum: -6, Maximum: 35, Copies: 2})
	if err != nil {
		return nil, err
	}
	palette, err := resource("palette.bin")
	if err != nil {
		return nil, err
	}
	colors := make([]color.NRGBA, 16)
	for i := range colors {
		w := binary.BigEndian.Uint16(palette[i*2:])
		colors[i] = color.NRGBA{R: byte(w>>8&7) * 34, G: byte(w>>4&7) * 34, B: byte(w&7) * 34, A: 255}
	}
	g.lookup, err = composite.NewIndexedPalette(composite.IndexedPaletteConfig{Palette: colors, Channel: composite.BitplaneRed, Blend: ebiten.BlendCopy})
	if err != nil {
		return nil, err
	}
	rows := make([]int, 200)
	for y := range rows {
		row := y % 25 / 3
		if row > 6 {
			row = 7
		}
		rows[y] = row
	}
	g.grid, err = composite.NewPaletteGrid(composite.PaletteGridConfig{Width: Width, Height: Height, Columns: 35, Rows: 8, X: composite.PaletteGridAxis{CellSize: 8, Offset: -16}, Y: composite.PaletteGridAxis{Indices: rows}, ControlChannel: composite.BitplaneAlpha, ControlThreshold: .5})
	if err != nil {
		return nil, err
	}
	g.stage = ebiten.NewImage(Width, Height)
	music, err := resource("music.ym")
	if err != nil {
		return nil, err
	}
	g.visual, err = sound.Open("music.ym", music, sound.Options{SampleRate: 48000, BlockFrames: 960, Loop: true})
	if err != nil {
		return nil, err
	}
	if !mute {
		g.player, err = playback.Open(nil, "music.ym", music, sound.Options{SampleRate: 48000, BlockFrames: 960, Loop: true})
		if err != nil {
			return nil, err
		}
		g.player.Play()
	}
	return g, nil
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	g.tick++
	if _, err := io.ReadFull(g.visual, g.pcm[:]); err != nil {
		return err
	}
	// Palette cells are sampled from the same literal font and bounded text
	// layout as DCK's ordinary image scroller, without a GPU readback.
	origin := 41 - float64(g.tick)
	if origin < -g.scroll.Length() {
		origin = math.Mod(origin, g.scroll.Length())
	}
	state := g.window.At(origin)
	clear(g.colors[:])
	state.Paint = func(_ *ebiten.Image, sample scrolling.Sample, _ ebiten.DrawImageOptions) {
		glyph := int(sample.Glyph.Rune) - 64
		if glyph < 0 || glyph >= 27 {
			return
		}
		left := int(sample.X)
		for col := 0; col < 6; col++ {
			x := left + col
			if x < 0 || x >= 35 {
				continue
			}
			for row := 0; row < 7; row++ {
				at := (row*27*6 + glyph*6 + col) * 4
				g.colors[row*35+x] = color.NRGBA{R: g.fontPixels[at], G: g.fontPixels[at+1], B: g.fontPixels[at+2], A: 255}
			}
		}
	}
	g.scroll.DrawAt(g.stage, state)
	if err := g.scroll.Err(); err != nil {
		return err
	}

	frame := kit.Frame{Tick: uint64(g.tick), Time: float64(g.tick) / FPS, Delta: 1.0 / FPS}
	if err := g.scroll.Update(frame); err != nil {
		return err
	}
	return g.grid.SetColors(g.transparent[:], g.colors[:])
}
func (g *Game) Draw(dst *ebiten.Image) {
	_ = g.lookup.Draw(dst, g.background)
	_ = g.grid.Draw(dst, g.mask, nil, composite.PaletteGridState{})
}
func (*Game) Layout(int, int) (int, int) { return Width, Height }
func (g *Game) Tick() int                { return g.tick }
func (g *Game) Close() {
	if g == nil || g.closed {
		return
	}
	g.closed = true
	if g.player != nil {
		g.player.Close()
	}
	if g.visual != nil {
		g.visual.Close()
	}
	if g.lookup != nil {
		g.lookup.Close()
	}
	if g.grid != nil {
		g.grid.Close()
	}
	if g.scroll != nil {
		g.scroll.Close()
	}
	for _, im := range append(g.images, g.stage) {
		if im != nil {
			im.Deallocate()
		}
	}
}
