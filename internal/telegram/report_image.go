package telegram

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"strings"
	"time"

	"funpay-parser/internal/models"
	"funpay-parser/internal/runner"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/NotoSans-Regular.ttf assets/NotoSans-SemiBold.ttf assets/NotoSans-Bold.ttf
var reportFonts embed.FS

const (
	reportW = 1708
	reportH = 954
)

var parsedReportFonts = map[string]*opentype.Font{}

func DealReportImage(res runner.Result) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, reportW, reportH))
	drawReferenceBackground(img)
	drawReferenceHeader(img, time.Now())
	drawReferenceDealCard(img, res)
	drawReferenceStats(img, res)
	drawReferenceFooter(img)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawReferenceBackground(img *image.RGBA) {
	for y := 0; y < reportH; y++ {
		for x := 0; x < reportW; x++ {
			nx := float64(x) / reportW
			ny := float64(y) / reportH
			vignette := math.Hypot(nx-0.50, ny-0.45)
			mid := math.Max(0, 1-vignette*1.25)
			r := uint8(7 + 9*mid)
			g := uint8(7 + 9*mid)
			b := uint8(8 + 11*mid)
			img.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	// vertical architectural grid like the reference.
	for x := 42; x < reportW; x += 31 {
		a := uint8(18)
		if x%124 == 42 {
			a = 26
		}
		drawLine(img, x, 0, x, reportH, color.RGBA{255, 255, 255, a})
	}
	for y := 0; y < reportH; y++ {
		shade := uint8(float64(y) / reportH * 28)
		for x := 0; x < reportW; x++ {
			blend(img, x, y, color.RGBA{0, 0, 0, shade})
		}
	}
	drawGlow(img, 270, 145, 230, color.RGBA{255, 255, 255, 12})
	drawGlow(img, 1490, 110, 250, color.RGBA{128, 128, 160, 14})
	r := rand.New(rand.NewSource(11))
	for i := 0; i < 180; i++ {
		x, y := r.Intn(reportW), r.Intn(reportH)
		if r.Float64() < 0.65 {
			blend(img, x, y, color.RGBA{255, 255, 255, uint8(20 + r.Intn(45))})
		}
	}
}

func drawReferenceHeader(img *image.RGBA, now time.Time) {
	white := color.RGBA{246, 246, 248, 255}
	muted := color.RGBA{168, 168, 174, 255}
	// logo block
	drawRoundedRect(img, 92, 76, 98, 98, 18, color.RGBA{17, 17, 18, 210}, color.RGBA{255, 255, 255, 60})
	drawPlanetLogo(img, 141, 125, 1.05, color.RGBA{246, 246, 248, 245})
	drawText(img, 224, 122, "FUNPAY PARSER", white, 44, "bold")
	drawText(img, 224, 162, "отчёт о парсинге предложений", muted, 25, "regular")
	// report pill
	drawRoundedRect(img, 1437, 76, 178, 56, 16, color.RGBA{15, 15, 16, 185}, color.RGBA{255, 255, 255, 52})
	drawTextCentered(img, 1437, 113, 178, "REPORT", color.RGBA{220, 220, 224, 255}, 22, "semibold")
	drawText(img, 1439, 165, now.Format("02.01.2006 15:04"), muted, 23, "regular")
}

func drawReferenceDealCard(img *image.RGBA, res runner.Result) {
	cardX, cardY, cardW, cardH := 92, 222, 1524, 386
	white := color.RGBA{245, 245, 248, 255}
	muted := color.RGBA{169, 169, 174, 255}
	line := color.RGBA{255, 255, 255, 54}
	drawRoundedRect(img, cardX, cardY, cardW, cardH, 14, color.RGBA{18, 18, 19, 198}, color.RGBA{255, 255, 255, 46})
	drawLine(img, cardX+49, cardY+226, cardX+1002, cardY+226, line)
	drawLine(img, cardX+1048, cardY+44, cardX+1048, cardY+340, line)
	drawLine(img, cardX+370, cardY+257, cardX+370, cardY+347, color.RGBA{255, 255, 255, 36})

	if res.Cheapest == nil {
		drawText(img, cardX+50, cardY+125, "Персональный аккаунт не найден", white, 48, "semibold")
		drawText(img, cardX+50, cardY+174, "Увеличь число кандидатов или включи Deep mode", muted, 28, "regular")
		return
	}
	l := res.Cheapest
	title := strings.TrimSpace(l.Title)
	if title == "" {
		title = "Найденное предложение"
	}
	sub := dealSubtitle(l)
	drawText(img, cardX+50, cardY+68, "НАЙДЕНО ПРЕДЛОЖЕНИЙ", muted, 20, "semibold")
	drawTextWrapped(img, cardX+50, cardY+142, title, white, 55, "semibold", 885, 64, 1)
	drawTextWrapped(img, cardX+50, cardY+195, sub, muted, 31, "regular", 900, 38, 1)

	drawText(img, cardX+50, cardY+275, "МИНИМАЛЬНАЯ ЦЕНА", muted, 20, "semibold")
	drawText(img, cardX+50, cardY+333, fmt.Sprintf("%.2f %s", l.Price, strings.TrimSpace(l.Currency)), color.RGBA{224, 224, 228, 255}, 46, "regular")
	drawText(img, cardX+440, cardY+275, "ПРОДАВЕЦ", muted, 20, "semibold")
	drawText(img, cardX+440, cardY+331, emptyDash(l.Seller), color.RGBA{224, 224, 228, 255}, 30, "regular")

	confX := cardX + 1115
	drawText(img, confX, cardY+116, "УВЕРЕННОСТЬ", muted, 20, "semibold")
	drawText(img, confX, cardY+220, confidence(l), color.RGBA{226, 226, 230, 255}, 88, "regular")
	barX, barY, barW := confX, cardY+266, 318
	drawRoundedRect(img, barX, barY, barW, 11, 5, color.RGBA{255, 255, 255, 34}, color.RGBA{255, 255, 255, 18})
	fillW := int(float64(barW) * confidenceValue(l))
	if fillW < 10 {
		fillW = 10
	}
	drawRoundedRect(img, barX, barY, fillW, 11, 5, color.RGBA{226, 226, 230, 220}, color.RGBA{255, 255, 255, 70})
}

func drawReferenceStats(img *image.RGBA, res runner.Result) {
	s := res.Summary
	stats := []struct {
		Label string
		Value int
		Icon  string
		Bar   float64
	}{
		{"ПРЕДЛОЖЕНИЙ", s["total_plus"], "stack", 0.18},
		{"ПРОДАВЦОВ", s["classified"], "user", 0.14},
		{"АККАУНТОВ", s["personal"], "shield", 0.20},
		{"ТРЕБУЮТ ПРОВЕРКИ", s["shared"], "alert", 0.18},
		{"ОШИБКИ", s["other"], "x", 0.15},
	}
	xs := []int{92, 408, 714, 1008, 1326}
	for i, st := range stats {
		drawStatCard(img, xs[i], 646, 292, 166, st.Label, st.Value, st.Icon, st.Bar)
	}
}

func drawStatCard(img *image.RGBA, x, y, w, h int, label string, value int, icon string, bar float64) {
	muted := color.RGBA{176, 176, 180, 255}
	white := color.RGBA{225, 225, 229, 255}
	drawRoundedRect(img, x, y, w, h, 14, color.RGBA{17, 17, 18, 196}, color.RGBA{255, 255, 255, 46})
	drawStatIcon(img, x+48, y+60, icon, color.RGBA{230, 230, 234, 230})
	drawText(img, x+91, y+50, label, muted, 17, "semibold")
	drawText(img, x+91, y+105, fmt.Sprintf("%d", value), white, 43, "regular")
	barX, barY, barW := x+28, y+135, w-56
	drawLine(img, barX, barY, barX+barW, barY, color.RGBA{255, 255, 255, 38})
	drawLine(img, barX, barY, barX+int(float64(barW)*bar), barY, color.RGBA{255, 255, 255, 230})
	drawLine(img, barX, barY+1, barX+int(float64(barW)*bar), barY+1, color.RGBA{255, 255, 255, 180})
}

func drawReferenceFooter(img *image.RGBA) {
	muted := color.RGBA{126, 126, 132, 255}
	x, y := 92, 865
	drawCircleOutline(img, x+13, y+1, 13, muted)
	drawTextCentered(img, x, y+8, 26, "i", muted, 18, "regular")
	drawText(img, x+44, y+8, "Данные актуальны на момент завершения парсинга", muted, 19, "regular")
	drawLine(img, x+560, y+4, 1616, y+4, color.RGBA{255, 255, 255, 34})
}

func dealSubtitle(l *models.Listing) string {
	kind := "Personal Account"
	if l.AccountType != nil && *l.AccountType != "" {
		kind = strings.Title(strings.ReplaceAll(*l.AccountType, "_", " ")) + " Account"
	}
	parts := []string{kind}
	if strings.Contains(strings.ToLower(l.Title), "codex") || strings.Contains(strings.ToLower(l.Description), "codex") {
		parts = append(parts, "Codex")
	}
	return strings.Join(parts, "  +  ")
}

func reportFont(weight string, size float64) font.Face {
	name := "NotoSans-Regular.ttf"
	switch weight {
	case "bold":
		name = "NotoSans-Bold.ttf"
	case "semibold":
		name = "NotoSans-SemiBold.ttf"
	}
	f := parsedReportFonts[name]
	if f == nil {
		b, err := reportFonts.ReadFile("assets/" + name)
		if err == nil {
			f, _ = opentype.Parse(b)
			parsedReportFonts[name] = f
		}
	}
	if f == nil {
		return nil
	}
	face, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	return face
}

func drawText(img *image.RGBA, x, y int, text string, col color.RGBA, size float64, weight string) {
	face := reportFont(weight, size)
	if face == nil || text == "" {
		return
	}
	d := &font.Drawer{Dst: img, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func textWidth(text string, size float64, weight string) int {
	face := reportFont(weight, size)
	if face == nil || text == "" {
		return 0
	}
	d := &font.Drawer{Face: face}
	return d.MeasureString(text).Round()
}

func drawTextCentered(img *image.RGBA, x, y, w int, text string, col color.RGBA, size float64, weight string) {
	drawText(img, x+(w-textWidth(text, size, weight))/2, y, text, col, size, weight)
}

func drawTextWrapped(img *image.RGBA, x, y int, text string, col color.RGBA, size float64, weight string, width, lineH, maxLines int) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return
	}
	line := ""
	lines := 0
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if textWidth(candidate, size, weight) > width && line != "" {
			drawText(img, x, y+lines*lineH, line, col, size, weight)
			lines++
			line = word
			if lines >= maxLines {
				return
			}
		} else {
			line = candidate
		}
	}
	if line != "" && lines < maxLines {
		drawText(img, x, y+lines*lineH, line, col, size, weight)
	}
}

func drawPlanetLogo(img *image.RGBA, cx, cy int, scale float64, c color.RGBA) {
	fillCircle(img, cx, cy, int(18*scale), c)
	for a := 0.0; a < math.Pi*2; a += 0.005 {
		rx := 34 * scale * math.Cos(a)
		ry := 10 * scale * math.Sin(a)
		px := cx + int(rx*math.Cos(-0.55)-ry*math.Sin(-0.55))
		py := cy + int(rx*math.Sin(-0.55)+ry*math.Cos(-0.55))
		fillCircle(img, px, py, max(1, int(2*scale)), c)
	}
	fillCircle(img, cx+21, cy-13, int(4*scale), color.RGBA{10, 10, 11, 255})
}

func drawStatIcon(img *image.RGBA, cx, cy int, icon string, c color.RGBA) {
	switch icon {
	case "stack":
		drawDiamond(img, cx, cy-12, 18, c)
		drawDiamond(img, cx, cy, 18, c)
		drawDiamond(img, cx, cy+12, 18, c)
	case "user":
		drawCircleOutline(img, cx, cy-11, 10, c)
		drawArcShoulders(img, cx, cy+22, 25, c)
	case "shield":
		drawShield(img, cx, cy, c)
	case "alert":
		drawCircleOutline(img, cx, cy, 18, c)
		drawLine(img, cx, cy-9, cx, cy+4, c)
		fillCircle(img, cx, cy+12, 2, c)
	case "x":
		drawCircleOutline(img, cx, cy, 18, c)
		drawLine(img, cx-8, cy-8, cx+8, cy+8, c)
		drawLine(img, cx+8, cy-8, cx-8, cy+8, c)
	}
}

func drawDiamond(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	drawLine(img, cx, cy-r, cx+r, cy, c)
	drawLine(img, cx+r, cy, cx, cy+r, c)
	drawLine(img, cx, cy+r, cx-r, cy, c)
	drawLine(img, cx-r, cy, cx, cy-r, c)
}
func drawArcShoulders(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for a := math.Pi; a <= math.Pi*2; a += 0.02 {
		x := cx + int(math.Cos(a)*float64(r))
		y := cy + int(math.Sin(a)*float64(r)*0.55)
		fillCircle(img, x, y, 1, c)
	}
}
func drawShield(img *image.RGBA, cx, cy int, c color.RGBA) {
	pts := [][2]int{{cx, cy - 22}, {cx + 17, cy - 14}, {cx + 14, cy + 10}, {cx, cy + 24}, {cx - 14, cy + 10}, {cx - 17, cy - 14}}
	for i := range pts {
		j := (i + 1) % len(pts)
		drawLine(img, pts[i][0], pts[i][1], pts[j][0], pts[j][1], c)
	}
	drawLine(img, cx-7, cy, cx-1, cy+7, c)
	drawLine(img, cx-1, cy+7, cx+10, cy-8, c)
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
	for i := 0; i < 1; i++ {
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
func drawCircleOutline(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for a := 0.0; a < math.Pi*2; a += 0.008 {
		fillCircle(img, cx+int(math.Cos(a)*float64(r)), cy+int(math.Sin(a)*float64(r)), 1, c)
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
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := int(math.Abs(float64(x1 - x0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -int(math.Abs(float64(y1 - y0)))
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		blend(img, x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
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
func confidenceValue(l *models.Listing) float64 {
	if l == nil || l.Confidence == nil {
		return 0
	}
	v := *l.Confidence
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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
