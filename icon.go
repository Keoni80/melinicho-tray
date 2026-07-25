package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/gomonobold"
)

var parsedFont *truetype.Font

func init() {
	f, err := freetype.ParseFont(gomonobold.TTF)
	if err != nil {
		panic(err) // la fuente está embebida, si esto falla es un bug de build
	}
	parsedFont = f
}

// formatCompact convierte un monto ARS en un string corto que entre en un
// ícono de bandeja chico: 531496 -> "$531K", 1234567 -> "$1.2M".
func formatCompact(amount float64) string {
	abs := math.Abs(amount)
	switch {
	case abs >= 1_000_000:
		return fmt.Sprintf("$%.1fM", amount/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("$%.0fK", amount/1_000)
	default:
		return fmt.Sprintf("$%.0f", amount)
	}
}

// renderIcon dibuja `text` centrado sobre un ícono transparente, en blanco
// con un leve contorno negro (para leerse tanto en bandejas claras como
// oscuras — no hay forma portable de saber el tema del panel en Linux/Windows).
// El ancho del ícono se ajusta al largo del texto.
func renderIcon(text string) []byte {
	const height = 44
	const fontSize = 30
	const padding = 6

	face := truetype.NewFace(parsedFont, &truetype.Options{
		Size:    fontSize,
		DPI:     72,
		Hinting: 0,
	})
	defer face.Close()

	// Medir el ancho del texto para dimensionar el canvas.
	var textWidth int
	for _, r := range text {
		aw, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}
		textWidth += aw.Round()
	}
	width := textWidth + padding*2
	if width < height {
		width = height
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fondo transparente.
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	baseline := height/2 + fontSize/3

	drawText := func(dst draw.Image, c color.Color, dx, dy int) {
		ctx := freetype.NewContext()
		ctx.SetDPI(72)
		ctx.SetFont(parsedFont)
		ctx.SetFontSize(fontSize)
		ctx.SetClip(dst.Bounds())
		ctx.SetDst(dst)
		ctx.SetSrc(image.NewUniform(c))
		pt := freetype.Pt(padding+dx, baseline+dy)
		ctx.DrawString(text, pt)
	}

	// Contorno negro: el mismo texto desplazado 1px en las 8 direcciones.
	black := color.RGBA{0, 0, 0, 255}
	for _, off := range [][2]int{
		{-1, -1}, {0, -1}, {1, -1},
		{-1, 0}, {1, 0},
		{-1, 1}, {0, 1}, {1, 1},
	} {
		drawText(img, black, off[0], off[1])
	}
	// Relleno blanco encima.
	drawText(img, color.White, 0, 0)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// renderTemplateIcon es una variante solo-negro-sobre-transparente para
// macOS (SetTemplateIcon), donde el propio SO invierte el color según el
// tema claro/oscuro de la barra de menús — un contorno blanco fijo se vería
// mal ahí, así que esta versión no lo lleva.
func renderTemplateIcon(text string) []byte {
	const height = 44
	const fontSize = 30
	const padding = 6

	face := truetype.NewFace(parsedFont, &truetype.Options{Size: fontSize, DPI: 72})
	defer face.Close()

	var textWidth int
	for _, r := range text {
		aw, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}
		textWidth += aw.Round()
	}
	width := textWidth + padding*2
	if width < height {
		width = height
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(parsedFont)
	ctx.SetFontSize(fontSize)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(color.Black))
	baseline := height/2 + fontSize/3
	ctx.DrawString(text, freetype.Pt(padding, baseline))

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
