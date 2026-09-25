package components

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"

	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// logoArt is the logo in half blocks, computed once.
var logoArt = sync.OnceValue(func() string { return halfBlocks(theme.Logo()) })

// halfBlocks draws two vertical pixels per cell (▀: fg = top, bg = bottom).
// A transparent pixel becomes a reset, which theme.Paint turns into the splash surface.
func halfBlocks(img image.Image) string {
	b := img.Bounds()
	var sb strings.Builder
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		if y > b.Min.Y {
			sb.WriteByte('\n')
		}
		for x := b.Min.X; x < b.Max.X; x++ {
			top, topOK := opaque(img.At(x, y))
			bot, botOK := color.RGBA{}, false
			if y+1 < b.Max.Y {
				bot, botOK = opaque(img.At(x, y+1))
			}
			switch {
			case topOK && botOK:
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", top.R, top.G, top.B, bot.R, bot.G, bot.B)
			case topOK:
				fmt.Fprintf(&sb, "\x1b[m\x1b[38;2;%d;%d;%dm▀", top.R, top.G, top.B)
			case botOK:
				fmt.Fprintf(&sb, "\x1b[m\x1b[38;2;%d;%d;%dm▄", bot.R, bot.G, bot.B)
			default:
				sb.WriteString("\x1b[m ")
			}
		}
		sb.WriteString("\x1b[m")
	}
	return sb.String()
}

// opaque returns the non-premultiplied color and whether the pixel is drawn.
func opaque(c color.Color) (color.RGBA, bool) {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return color.RGBA{R: n.R, G: n.G, B: n.B, A: 255}, n.A >= 128
}
