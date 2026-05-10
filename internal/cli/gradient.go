package cli

import (
	"fmt"
	"math"

	"github.com/charmbracelet/lipgloss"
)

type rgb struct{ r, g, b float64 }

func hexToRGB(hex string) rgb {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return rgb{float64(r), float64(g), float64(b)}
}

func lerpRGB(a, b rgb, t float64) rgb {
	return rgb{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

func (c rgb) hex() string {
	return fmt.Sprintf("#%02x%02x%02x", int(math.Round(c.r)), int(math.Round(c.g)), int(math.Round(c.b)))
}

func gradientMulti(text string, colors ...string) string {
	if len(colors) < 2 {
		if len(colors) == 1 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(colors[0])).Render(text)
		}
		return text
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}

	segments := len(colors) - 1
	rgbs := make([]rgb, len(colors))
	for i, c := range colors {
		rgbs[i] = hexToRGB(c)
	}

	result := ""
	for i, r := range runes {
		t := float64(i) / float64(max(len(runes)-1, 1))
		seg := int(t * float64(segments))
		if seg >= segments {
			seg = segments - 1
		}
		localT := t*float64(segments) - float64(seg)
		c := lerpRGB(rgbs[seg], rgbs[seg+1], localT)
		result += lipgloss.NewStyle().Foreground(lipgloss.Color(c.hex())).Render(string(r))
	}
	return result
}

var brandRGBs = func() []rgb {
	out := make([]rgb, len(brandColors))
	for i, c := range brandColors {
		out[i] = hexToRGB(c)
	}
	return out
}()

func multiLerpRGB(t float64, rgbs []rgb) rgb {
	if len(rgbs) < 2 {
		if len(rgbs) == 1 {
			return rgbs[0]
		}
		return rgb{255, 255, 255}
	}
	segments := len(rgbs) - 1
	seg := int(t * float64(segments))
	if seg >= segments {
		seg = segments - 1
	}
	localT := t*float64(segments) - float64(seg)
	return lerpRGB(rgbs[seg], rgbs[seg+1], localT)
}

func gradientBorder(style lipgloss.Style, colors ...string) lipgloss.Style {
	if len(colors) < 2 {
		return style
	}
	rgbs := make([]rgb, len(colors))
	for i, c := range colors {
		rgbs[i] = hexToRGB(c)
	}
	top := lipgloss.Color(colors[0])
	right := lipgloss.Color(lerpRGB(rgbs[0], rgbs[len(rgbs)-1], 0.33).hex())
	bottom := lipgloss.Color(lerpRGB(rgbs[0], rgbs[len(rgbs)-1], 0.66).hex())
	left := lipgloss.Color(colors[len(colors)-1])
	return style.
		BorderTopForeground(top).
		BorderRightForeground(right).
		BorderBottomForeground(bottom).
		BorderLeftForeground(left)
}
