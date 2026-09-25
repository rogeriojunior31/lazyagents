package theme

import (
	"image"

	"charm.land/lipgloss/v2"
)

// logoRows is the sloth in the hammock in pixel art, the terminal version of
// docs/assets/logo.svg. Each rune is a logoInk pixel; '.' is transparent.
// Colors are fixed: the logo does not follow the theme.
var logoRows = []string{
	".......f.f....yyyy............",
	"r.....fffff.....y............r",
	".r...fffffff...y.......cc...r.",
	"..r.ffcccccff.yyyy....FFcc.r..",
	"...rfcccccccf........FFF..r...",
	"....fpppcpppf.......FFF..r....",
	"....fpccnccpfffffffff...r.....",
	".....ooccccffeeeeeeffffoo.....",
	"......oooffffffffffffooo......",
	".......LLLLLLLLLLLLLLLL.......",
	".......oooooFoooooooooo.......",
	"........bbbbFbbbbbbbbb........",
	".........oooFoooooooo.........",
	"...........dFdddddd...........",
	"...........dFd.d.d............",
	"...........ccc................",
}

var logoInk = map[rune]string{
	'f': "#9b7653", // fur
	'F': "#7a5a3d", // fur in shadow, foot and arm
	'c': "#f3e3c3", // face and claws
	'e': "#e6d0a6", // belly
	'p': "#4a3426", // eye mask
	'n': "#2a1d15", // nose
	'o': "#f2984a", // hammock
	'L': "#ffba7c", // lit hammock edge
	'd': "#c4652a", // hammock in shadow and fringe
	'b': "#6e92de", // stripe
	'r': "#c9b08f", // rope
	'y': "#f5c66b", // sleep z
}

// Logo returns the logo pixels for terminal drawing.
func Logo() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, len(logoRows[0]), len(logoRows)))
	for y, row := range logoRows {
		for x, r := range row {
			if hex, ok := logoInk[r]; ok {
				img.Set(x, y, lipgloss.Color(hex))
			}
		}
	}
	return img
}
