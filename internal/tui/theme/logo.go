package theme

import (
	"image"

	"charm.land/lipgloss/v2"
)

// logoRows é a preguiça dormindo na rede em pixel-art, a versão de terminal de
// docs/assets/logo.svg. Cada rune é um pixel com a cor de logoInk; '.' é
// transparente. As cores são fixas: o logo não muda com o tema.
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
	'f': "#9b7653", // pelo
	'F': "#7a5a3d", // pelo na sombra, pé e braço
	'c': "#f3e3c3", // rosto e garras
	'e': "#e6d0a6", // barriga
	'p': "#4a3426", // máscara dos olhos
	'n': "#2a1d15", // nariz
	'o': "#f2984a", // rede
	'L': "#ffba7c", // borda iluminada da rede
	'd': "#c4652a", // rede na sombra e franja
	'b': "#6e92de", // listra
	'r': "#c9b08f", // corda
	'y': "#f5c66b", // z de sono
}

// Logo devolve o logo em pixels, para quem o desenha no terminal.
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
