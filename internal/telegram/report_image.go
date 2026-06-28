package telegram

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"strings"
	"time"

	"funpay-parser/internal/runner"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	reportW = 1200
	reportH = 675
)

func DealReportImage(res runner.Result) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, reportW, reportH))
	drawBackground(img)
	drawGlow(img, 190, 130, 280, color.RGBA{70, 125, 255, 58})
	drawGlow(img, 980, 115, 260, color.RGBA{156, 92, 255, 48})
	drawGlow(img, 650, 610, 360, color.RGBA{30, 211, 238, 34})
	drawStars(img)
	drawLogo(img, 78, 64)

	white := color.RGBA{245, 247, 255, 255}
	muted := color.RGBA{156, 163, 183, 255}
	accent := color.RGBA{255, 255, 255, 255}
	blue := color.RGBA{96, 165, 250, 255}
	green := color.RGBA{52, 211, 153, 255}
	red := color.RGBA{248, 113, 113, 255}

	drawText(img, 160, 80, "FUNPAY PARSER", white, 2)
	drawText(img, 162, 114, "dark space deal report", muted, 1)
	drawPill(img, 945, 64, 178, 42, "REPORT", color.RGBA{255, 255, 255, 230})
	drawText(img, 948, 130, time.Now().Format("02.01.2006 15:04"), muted, 1)

	card := color.RGBA{12, 13, 18, 226}
	drawRoundedRect(img, 64, 178, 1072, 330, 28, card, color.RGBA{255, 255, 255, 28})
	if res.Cheapest != nil {
		l := res.Cheapest
		drawText(img, 96, 220, "CHEAPEST PERSONAL DEAL", muted, 1)
		drawTextWrapped(img, 96, 268, truncate(l.Title, 94), white, 3, 860, 42, 2)
		price := fmt.Sprintf("%.2f %s", l.Price, strings.TrimSpace(l.Currency))
		drawText(img, 96, 405, price, green, 4)
		drawText(img, 100, 462, "seller: "+emptyDash(l.Seller), muted, 1)
		drawPill(img, 896, 392, 180, 44, "confidence "+confidence(l), blue)
	} else {
		drawText(img, 96, 260, "No personal account confirmed", red, 3)
		drawText(img, 100, 320, "Try more candidates or enable Deep mode", muted, 1)
	}

	s := res.Summary
	stats := []struct {
		Label string
		Value int
		Col   color.RGBA
	}{
		{"Listings", s["total_plus"], accent},
		{"LLM", s["classified"], blue},
		{"Personal", s["personal"], green},
		{"Shared", s["shared"], color.RGBA{251, 191, 36, 255}},
		{"Other", s["other"], muted},
	}
	startX := 64
	for i, st := range stats {
		x := startX + i*218
		drawRoundedRect(img, x, 540, 188, 84, 22, color.RGBA{14, 15, 21, 228}, color.RGBA{255, 255, 255, 22})
		drawTextCentered(img, x, 570, 188, fmt.Sprintf("%d", st.Value), st.Col, 3)
		drawTextCentered(img, x, 606, 188, st.Label, muted, 1)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawBackground(img *image.RGBA) {
	for y := 0; y < reportH; y++ {
		for x := 0; x < reportW; x++ {
			t := float64(y) / reportH
			cx := float64(x-reportW/2) / reportW
			cy := float64(y-reportH/2) / reportH
			v := math.Sqrt(cx*cx+cy*cy) * 1.3
			r := uint8(4 + 7*t + 7*math.Max(0, 1-v))
			g := uint8(5 + 8*t + 5*math.Max(0, 1-v))
			b := uint8(9 + 15*t + 22*math.Max(0, 1-v))
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
}

func drawStars(img *image.RGBA) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 420; i++ {
		x, y := r.Intn(reportW), r.Intn(reportH)
		a := uint8(70 + r.Intn(160))
		img.SetRGBA(x, y, color.RGBA{255, 255, 255, a})
		if r.Float64() < 0.12 && x+1 < reportW {
			img.SetRGBA(x+1, y, color.RGBA{255, 255, 255, a / 2})
		}
	}
}

func drawGlow(img *image.RGBA, cx, cy, radius int, col color.RGBA) {
	for y := max(0, cy-radius); y < min(reportH, cy+radius); y++ {
		for x := max(0, cx-radius); x < min(reportW, cx+radius); x++ {
			d := math.Hypot(float64(x-cx), float64(y-cy)) / float64(radius)
			if d > 1 {
				continue
			}
			a := float64(col.A) * (1 - d) * (1 - d)
			blend(img, x, y, color.RGBA{col.R, col.G, col.B, uint8(a)})
		}
	}
}

func drawLogo(img *image.RGBA, x, y int) {
	drawRoundedRect(img, x, y, 60, 60, 18, color.RGBA{245, 247, 255, 245}, color.RGBA{255, 255, 255, 70})
	cx, cy := x+30, y+30
	for a := 0.0; a < math.Pi*2; a += 0.01 {
		rx := 25 * math.Cos(a)
		ry := 8 * math.Sin(a)
		px := cx + int(rx*math.Cos(-0.62)-ry*math.Sin(-0.62))
		py := cy + int(rx*math.Sin(-0.62)+ry*math.Cos(-0.62))
		if px >= 0 && py >= 0 && px < reportW && py < reportH {
			img.SetRGBA(px, py, color.RGBA{5, 5, 6, 255})
		}
	}
	fillCircle(img, cx, cy, 10, color.RGBA{5, 5, 6, 255})
	fillCircle(img, x+47, y+18, 4, color.RGBA{5, 5, 6, 255})
}

func drawRoundedRect(img *image.RGBA, x, y, w, h, r int, fill, stroke color.RGBA) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			dx := max(max(x-xx+r, 0), max(xx-(x+w-r-1), 0))
			dy := max(max(y-yy+r, 0), max(yy-(y+h-r-1), 0))
			if dx*dx+dy*dy <= r*r {
				blend(img, xx, yy, fill)
			}
		}
	}
	for i := 0; i < 2; i++ {
		drawRectOutline(img, x+i, y+i, w-2*i, h-2*i, r, stroke)
	}
}

func drawRectOutline(img *image.RGBA, x, y, w, h, r int, c color.RGBA) {
	for xx := x + r; xx < x+w-r; xx++ {
		blend(img, xx, y, c)
		blend(img, xx, y+h-1, c)
	}
	for yy := y + r; yy < y+h-r; yy++ {
		blend(img, x, yy, c)
		blend(img, x+w-1, yy, c)
	}
}
func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				blend(img, x, y, c)
			}
		}
	}
}

func drawPill(img *image.RGBA, x, y, w, h int, text string, col color.RGBA) {
	drawRoundedRect(img, x, y, w, h, h/2, color.RGBA{255, 255, 255, 24}, color.RGBA{255, 255, 255, 34})
	drawTextCentered(img, x, y+h/2+5, w, text, col, 1)
}
func drawText(img *image.RGBA, x, y int, text string, col color.RGBA, scale int) {
	drawScaledText(img, x, y, text, col, scale)
}
func drawTextCentered(img *image.RGBA, x, y, w int, text string, col color.RGBA, scale int) {
	tw := len([]rune(text)) * 7 * scale
	drawScaledText(img, x+(w-tw)/2, y, text, col, scale)
}
func drawTextWrapped(img *image.RGBA, x, y int, text string, col color.RGBA, scale, width, lineH, maxLines int) {
	words := strings.Fields(text)
	line := ""
	lines := 0
	maxChars := width / (7 * scale)
	for _, w := range words {
		if len([]rune(line))+len([]rune(w))+1 > maxChars && line != "" {
			drawText(img, x, y+lines*lineH, line, col, scale)
			lines++
			line = w
			if lines >= maxLines {
				return
			}
		} else {
			if line != "" {
				line += " "
			}
			line += w
		}
	}
	if line != "" && lines < maxLines {
		drawText(img, x, y+lines*lineH, line, col, scale)
	}
}

func drawScaledText(img *image.RGBA, x, y int, text string, col color.RGBA, scale int) {
	if scale <= 1 {
		d := &font.Drawer{Dst: img, Src: image.NewUniform(col), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
		d.DrawString(text)
		return
	}
	tmp := image.NewRGBA(image.Rect(0, 0, len([]rune(text))*7+8, 16))
	d := &font.Drawer{Dst: tmp, Src: image.NewUniform(col), Face: basicfont.Face7x13, Dot: fixed.P(0, 13)}
	d.DrawString(text)
	for yy := 0; yy < tmp.Bounds().Dy(); yy++ {
		for xx := 0; xx < tmp.Bounds().Dx(); xx++ {
			_, _, _, a := tmp.At(xx, yy).RGBA()
			if a > 0 {
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						blend(img, x+xx*scale+sx, y-13*scale+yy*scale+sy, col)
					}
				}
			}
		}
	}
}

func blend(img *image.RGBA, x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= reportW || y >= reportH || c.A == 0 {
		return
	}
	dst := img.RGBAAt(x, y)
	a := float64(c.A) / 255
	img.SetRGBA(x, y, color.RGBA{uint8(float64(c.R)*a + float64(dst.R)*(1-a)), uint8(float64(c.G)*a + float64(dst.G)*(1-a)), uint8(float64(c.B)*a + float64(dst.B)*(1-a)), 255})
}
func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
func emptyDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
