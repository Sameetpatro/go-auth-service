package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	canvasWidth = 420
	qrSize      = 256
	horizPad    = 24
	vertPad     = 20
	lineGap     = 8
)

type cardLabels struct {
	title    string
	dateTime string
	location string
}

func renderInvitationCard(token string, labels cardLabels) ([]byte, error) {
	qrPNG, err := qrcode.Encode(token, qrcode.Medium, qrSize)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}

	qrImg, err := png.Decode(bytes.NewReader(qrPNG))
	if err != nil {
		return nil, fmt.Errorf("decode qr: %w", err)
	}

	titleFace, err := loadFontFace(gobold.TTF, 15)
	if err != nil {
		return nil, err
	}
	bodyFace, err := loadFontFace(goregular.TTF, 13)
	if err != nil {
		return nil, err
	}

	titleLines := wrapText(labels.title, titleFace, canvasWidth-2*horizPad)
	bodyLines := append(
		wrapText(labels.dateTime, bodyFace, canvasWidth-2*horizPad),
		wrapText(labels.location, bodyFace, canvasWidth-2*horizPad)...,
	)

	titleBlockHeight := textBlockHeight(titleLines, titleFace, lineGap)
	bodyBlockHeight := textBlockHeight(bodyLines, bodyFace, lineGap)
	textGap := 16

	canvasHeight := vertPad + qrSize + textGap + titleBlockHeight + lineGap + bodyBlockHeight + vertPad

	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	qrX := (canvasWidth - qrSize) / 2
	draw.Draw(canvas, image.Rect(qrX, vertPad, qrX+qrSize, vertPad+qrSize), qrImg, image.Point{}, draw.Over)

	textY := vertPad + qrSize + textGap
	textY = drawCenteredLines(canvas, titleLines, titleFace, textY, lineGap)
	textY += lineGap
	drawCenteredLines(canvas, bodyLines, bodyFace, textY, lineGap)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("encode card png: %w", err)
	}
	return buf.Bytes(), nil
}

func writeInvitationCard(path, token string, labels cardLabels) error {
	data, err := renderInvitationCard(token, labels)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadFontFace(ttf []byte, size float64) (font.Face, error) {
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     96,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("new font face: %w", err)
	}
	return face, nil
}

func wrapText(text string, face font.Face, maxWidth int) []string {
	if text == "" {
		return nil
	}

	words := splitWords(text)
	var lines []string
	var current string

	for _, word := range words {
		candidate := current
		if candidate != "" {
			candidate += " "
		}
		candidate += word

		if textWidth(candidate, face) <= maxWidth {
			current = candidate
			continue
		}

		if current != "" {
			lines = append(lines, current)
			current = word
			continue
		}

		lines = append(lines, word)
		current = ""
	}

	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitWords(text string) []string {
	var words []string
	start := -1
	for i, r := range text {
		if r == ' ' {
			if start >= 0 {
				words = append(words, text[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		words = append(words, text[start:])
	}
	return words
}

func textWidth(text string, face font.Face) int {
	advance := font.MeasureString(face, text)
	return advance.Ceil()
}

func textBlockHeight(lines []string, face font.Face, gap int) int {
	if len(lines) == 0 {
		return 0
	}
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	return len(lines)*lineHeight + (len(lines)-1)*gap
}

func drawCenteredLines(dst *image.RGBA, lines []string, face font.Face, startY, gap int) int {
	if len(lines) == 0 {
		return startY
	}

	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	y := startY + metrics.Ascent.Ceil()

	for i, line := range lines {
		width := textWidth(line, face)
		x := (canvasWidth - width) / 2
		drawer := &font.Drawer{
			Dst:  dst,
			Src:  image.NewUniform(color.Black),
			Face: face,
			Dot:  fixed.P(x, y),
		}
		drawer.DrawString(line)
		y += lineHeight
		if i < len(lines)-1 {
			y += gap
		}
	}

	return y
}
