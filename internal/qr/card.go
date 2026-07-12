package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"regexp"
	"strings"
	"unicode"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/sameetpatro/go-qr-auth/internal/tags"
)

const (
	canvasWidth     = 480
	qrSize          = 220
	horizPad        = 28
	vertPad         = 24
	lineGap         = 6
	headerHeight    = 72
	footerPad       = 20
	tagBannerHeight = 34
)

var (
	colorPrimary = color.RGBA{R: 26, G: 35, B: 126, A: 255}   // deep indigo
	colorAccent  = color.RGBA{R: 255, G: 193, B: 7, A: 255}   // gold accent
	colorBg      = color.RGBA{R: 250, G: 251, B: 255, A: 255} // soft white
	colorText    = color.RGBA{R: 33, G: 33, B: 33, A: 255}
	colorMuted   = color.RGBA{R: 97, G: 97, B: 97, A: 255}
	colorDivider = color.RGBA{R: 224, G: 224, B: 224, A: 255}
)

type GuestCardInfo struct {
	Name          string
	Phone         string
	Email         string
	Address       string
	Department    string
	EventName     string
	EventDate     string
	EventLocation string
	// Tag is the raw/normalized member tag key (e.g. "vip"); empty defaults to invitee.
	Tag string
}

func renderInvitationCard(token string, info GuestCardInfo) ([]byte, error) {
	qrPNG, err := qrcode.Encode(token, qrcode.Medium, qrSize)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}

	qrImg, err := png.Decode(bytes.NewReader(qrPNG))
	if err != nil {
		return nil, fmt.Errorf("decode qr: %w", err)
	}

	nameFace, err := loadFontFace(gobold.TTF, 20)
	if err != nil {
		return nil, err
	}
	subtitleFace, err := loadFontFace(goregular.TTF, 12)
	if err != nil {
		return nil, err
	}
	bodyFace, err := loadFontFace(goregular.TTF, 11)
	if err != nil {
		return nil, err
	}
	labelFace, err := loadFontFace(gobold.TTF, 10)
	if err != nil {
		return nil, err
	}

	tag := tags.Get(info.Tag)
	tagLabelFace, err := loadFontFace(gobold.TTF, 15)
	if err != nil {
		return nil, err
	}

	detailLines := buildDetailLines(info, labelFace, bodyFace, canvasWidth-2*horizPad)
	detailBlockHeight := textBlockHeight(detailLines, bodyFace, lineGap) + 8

	eventLines := append(
		wrapText(info.EventName, subtitleFace, canvasWidth-2*horizPad),
		wrapText(info.EventDate, bodyFace, canvasWidth-2*horizPad)...,
	)
	eventBlockHeight := textBlockHeight(eventLines, bodyFace, lineGap)

	canvasHeight := vertPad + headerHeight + tagBannerHeight + qrSize + 20 + eventBlockHeight + 16 + detailBlockHeight + footerPad + vertPad

	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{colorBg}, image.Point{}, draw.Src)

	// Header band
	draw.Draw(canvas, image.Rect(0, 0, canvasWidth, headerHeight), &image.Uniform{colorPrimary}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(0, headerHeight-4, canvasWidth, headerHeight), &image.Uniform{tag.Color}, image.Point{}, draw.Src)

	// Guest name in header
	nameLines := wrapText(strings.ToUpper(info.Name), nameFace, canvasWidth-2*horizPad)
	nameY := (headerHeight - textBlockHeight(nameLines, nameFace, 4)) / 2
	drawCenteredLines(canvas, nameLines, nameFace, nameY, 4, color.White)

	// Tag banner in the member's color, right below the header.
	bannerTop := headerHeight
	bannerBottom := headerHeight + tagBannerHeight
	draw.Draw(canvas, image.Rect(0, bannerTop, canvasWidth, bannerBottom), &image.Uniform{tag.Color}, image.Point{}, draw.Src)
	// A thin darker underline keeps white/light banners visually separated from the card body.
	draw.Draw(canvas, image.Rect(0, bannerBottom-2, canvasWidth, bannerBottom), &image.Uniform{tags.Darken(tag.Color, 0.7)}, image.Point{}, draw.Src)
	tagText := strings.ToUpper(tag.Display)
	tagTextW := textWidth(tagText, tagLabelFace)
	tagTextX := (canvasWidth - tagTextW) / 2
	tagMetrics := tagLabelFace.Metrics()
	tagTextY := bannerTop + (tagBannerHeight+tagMetrics.Ascent.Ceil()-tagMetrics.Descent.Ceil())/2
	tagDrawer := &font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(tags.TextColor(tag.Color)),
		Face: tagLabelFace,
		Dot:  fixed.P(tagTextX, tagTextY),
	}
	tagDrawer.DrawString(tagText)

	// QR code with border (kept black-on-white so scanners read it reliably;
	// the member color is conveyed by the header/banner/border instead).
	qrY := bannerBottom + vertPad
	qrX := (canvasWidth - qrSize) / 2
	borderPad := 8
	draw.Draw(canvas,
		image.Rect(qrX-borderPad, qrY-borderPad, qrX+qrSize+borderPad, qrY+qrSize+borderPad),
		&image.Uniform{color.White}, image.Point{}, draw.Src)
	draw.Draw(canvas,
		image.Rect(qrX-borderPad, qrY-borderPad, qrX+qrSize+borderPad, qrY+qrSize+borderPad),
		&image.Uniform{tags.Darken(tag.Color, 0.85)}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(qrX, qrY, qrX+qrSize, qrY+qrSize), qrImg, image.Point{}, draw.Over)

	// Event info below QR
	textY := qrY + qrSize + 20
	textY = drawCenteredLines(canvas, eventLines, bodyFace, textY, lineGap, colorText)

	// Divider
	divY := textY + 12
	draw.Draw(canvas, image.Rect(horizPad, divY, canvasWidth-horizPad, divY+1), &image.Uniform{colorDivider}, image.Point{}, draw.Src)

	// Guest details at bottom
	textY = divY + 14
	drawLeftLines(canvas, detailLines, bodyFace, horizPad, textY, lineGap, colorMuted)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("encode card png: %w", err)
	}
	return buf.Bytes(), nil
}

func buildDetailLines(info GuestCardInfo, labelFace, bodyFace font.Face, maxWidth int) []string {
	var lines []string
	addField := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, value))
	}
	addField("Phone", info.Phone)
	addField("Email", info.Email)
	addField("Address", info.Address)
	addField("Department", info.Department)

	if len(lines) == 0 {
		lines = append(lines, "Invitation Card")
	}

	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, wrapText(line, bodyFace, maxWidth)...)
	}
	return wrapped
}

func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('_')
		}
	}
	result := b.String()
	result = regexp.MustCompile(`_+`).ReplaceAllString(result, "_")
	result = strings.Trim(result, "_")
	if result == "" {
		return "guest"
	}
	if len(result) > 40 {
		result = result[:40]
	}
	return result
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
	return font.MeasureString(face, text).Ceil()
}

func textBlockHeight(lines []string, face font.Face, gap int) int {
	if len(lines) == 0 {
		return 0
	}
	lineHeight := face.Metrics().Height.Ceil()
	return len(lines)*lineHeight + (len(lines)-1)*gap
}

func drawCenteredLines(dst *image.RGBA, lines []string, face font.Face, startY, gap int, col color.Color) int {
	if len(lines) == 0 {
		return startY
	}
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	y := startY + metrics.Ascent.Ceil()
	for i, line := range lines {
		width := textWidth(line, face)
		x := (canvasWidth - width) / 2
		drawer := &font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
		drawer.DrawString(line)
		y += lineHeight
		if i < len(lines)-1 {
			y += gap
		}
	}
	return y
}

func drawLeftLines(dst *image.RGBA, lines []string, face font.Face, x, startY, gap int, col color.Color) int {
	if len(lines) == 0 {
		return startY
	}
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	y := startY + metrics.Ascent.Ceil()
	for i, line := range lines {
		drawer := &font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
		drawer.DrawString(line)
		y += lineHeight
		if i < len(lines)-1 {
			y += gap
		}
	}
	return y
}
