package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"runtime"
	"strconv"
	"strings"

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

// formatThousands da formato "es-AR" a un monto entero: separador de miles
// con punto, sin decimales (14223727 -> "14.223.727") — para tooltip,
// overlay y menú, donde sí entra el número completo (a diferencia del
// ícono de bandeja, que usa formatCompact()).
func formatThousands(amount float64) string {
	n := int64(math.Round(amount))
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		out = "-" + out
	}
	return out
}

// formatCompact convierte un monto ARS en un string corto que entre en un
// ícono de bandeja chico: 531496 -> "$531K", 1234567 -> "$1.23M".
func formatCompact(amount float64) string {
	abs := math.Abs(amount)
	switch {
	case abs >= 1_000_000:
		return fmt.Sprintf("$%.2fM", amount/1_000_000)
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
	const height = 64
	const fontSize = 46
	const padding = 10

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
	pngBytes := buf.Bytes()
	if runtime.GOOS == "windows" {
		// systray.SetIcon en Windows carga el archivo con LoadImage(...,
		// IMAGE_ICON, ..., LR_LOADFROMFILE), que solo entiende el contenedor
		// .ico — un PNG suelto falla en silencio (LoadImage devuelve NULL) y
		// el ícono nunca aparece en la bandeja. Envolver el PNG en un .ico
		// mínimo (formato "PNG icon", soportado desde Windows Vista) alcanza,
		// sin necesidad de reimplementar un encoder BMP.
		return wrapICO(pngBytes, width, height)
	}
	return pngBytes
}

// wrapICO envuelve una imagen PNG en un contenedor .ico de un solo frame.
func wrapICO(pngBytes []byte, width, height int) []byte {
	var buf bytes.Buffer

	bWidth := byte(width)
	if width >= 256 {
		bWidth = 0 // 0 significa 256 en el formato ICO
	}
	bHeight := byte(height)
	if height >= 256 {
		bHeight = 0
	}

	// ICONDIR (6 bytes)
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reservado
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // tipo: ícono
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // 1 imagen

	// ICONDIRENTRY (16 bytes)
	buf.WriteByte(bWidth)
	buf.WriteByte(bHeight)
	buf.WriteByte(0)                                                // sin paleta
	buf.WriteByte(0)                                                // reservado
	binary.Write(&buf, binary.LittleEndian, uint16(1))              // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))             // bits por píxel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngBytes)))  // tamaño de la imagen
	binary.Write(&buf, binary.LittleEndian, uint32(6+16))           // offset: ICONDIR + 1 ICONDIRENTRY

	buf.Write(pngBytes)
	return buf.Bytes()
}

// renderTemplateIcon es una variante solo-negro-sobre-transparente para
// macOS (SetTemplateIcon), donde el propio SO invierte el color según el
// tema claro/oscuro de la barra de menús — un contorno blanco fijo se vería
// mal ahí, así que esta versión no lo lleva.
func renderTemplateIcon(text string) []byte {
	const height = 64
	const fontSize = 46
	const padding = 10

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
