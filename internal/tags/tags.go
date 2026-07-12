// Package tags defines the canonical member tags (VIP, VVIP, Media, ...) used
// across the QR invitation card and analytics. Tags are stored in the guest
// metadata JSON under the "tag" key; guests without a tag default to "invitee".
package tags

import (
	"fmt"
	"image/color"
	"strings"
)

// DefaultKey is applied whenever a guest has no tag set.
const DefaultKey = "invitee"

// Tag describes a member category and its brand color.
type Tag struct {
	Key     string     // canonical lowercase identifier stored in metadata
	Display string     // human-readable label shown on the card / app
	Color   color.RGBA // accent color for the card header + badge
}

// all is the ordered catalogue of supported tags.
var all = []Tag{
	{Key: "vvip", Display: "VVIP", Color: color.RGBA{R: 0xD9, G: 0xDC, B: 0xE1, A: 0xFF}},           // light gray
	{Key: "vip", Display: "VIP", Color: color.RGBA{R: 0x8F, G: 0xD8, B: 0xF0, A: 0xFF}},             // ice blue
	{Key: "organiser", Display: "Organiser", Color: color.RGBA{R: 0xD4, G: 0xAF, B: 0x37, A: 0xFF}}, // gold
	{Key: "awardee", Display: "Awardee", Color: color.RGBA{R: 0x9C, G: 0x27, B: 0xB0, A: 0xFF}},     // purple
	{Key: "core_team", Display: "Core Team", Color: color.RGBA{R: 0xFF, G: 0xEB, B: 0x3B, A: 0xFF}}, // yellow
	{Key: "media", Display: "Media", Color: color.RGBA{R: 0x43, G: 0xA0, B: 0x47, A: 0xFF}},         // green
	{Key: "volunteer", Display: "Volunteer", Color: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}, // white
	{Key: "invitee", Display: "Invitee", Color: color.RGBA{R: 0xFB, G: 0x8C, B: 0x00, A: 0xFF}},     // orange
}

var byKey = func() map[string]Tag {
	m := make(map[string]Tag, len(all))
	for _, t := range all {
		m[t.Key] = t
	}
	return m
}()

// aliases maps common spreadsheet / user spellings to canonical keys.
var aliases = map[string]string{
	"v.v.i.p":    "vvip",
	"v.i.p":      "vip",
	"core team":  "core_team",
	"coreteam":   "core_team",
	"core-team":  "core_team",
	"organizer":  "organiser",
	"organiser":  "organiser",
	"guest":      "invitee",
	"invited":    "invitee",
	"invitees":   "invitee",
	"press":      "media",
	"volunteers": "volunteer",
	"awardees":   "awardee",
	"award":      "awardee",
}

// Hex returns the tag color as a "#RRGGBB" string.
func (t Tag) Hex() string {
	return fmt.Sprintf("#%02X%02X%02X", t.Color.R, t.Color.G, t.Color.B)
}

// All returns the ordered catalogue of tags.
func All() []Tag { return all }

// Normalize maps any user-supplied value to a canonical tag key, defaulting to
// "invitee" when the value is empty or unrecognized.
func Normalize(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return DefaultKey
	}
	if _, ok := byKey[s]; ok {
		return s
	}
	// Collapse separators to match aliases like "core team" / "core-team".
	if canonical, ok := aliases[s]; ok {
		return canonical
	}
	compact := strings.NewReplacer("-", "_", " ", "_", ".", "").Replace(s)
	if _, ok := byKey[compact]; ok {
		return compact
	}
	if canonical, ok := aliases[compact]; ok {
		return canonical
	}
	return DefaultKey
}

// Get returns the Tag for a raw value (normalized), always non-empty.
func Get(raw string) Tag {
	return byKey[Normalize(raw)]
}

// Display returns the human-readable label for a raw value.
func Display(raw string) string { return Get(raw).Display }

// TextColor returns black or white depending on the tag color luminance so the
// badge label stays readable on light (white/gold/ice-blue) vs dark backgrounds.
func TextColor(c color.RGBA) color.RGBA {
	// Perceived luminance (ITU-R BT.601).
	lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
	if lum > 150 {
		return color.RGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
	}
	return color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
}

// Darken returns a darker shade of c for outlines/dividers on light colors.
func Darken(c color.RGBA, factor float64) color.RGBA {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return color.RGBA{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
		A: 0xFF,
	}
}
