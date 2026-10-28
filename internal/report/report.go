// Package report generates PDF, CSV and QR exports of matches and competitions.
package report

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"

	"github.com/janenen/auswertung/internal/model"
)

// MatchInfo is a match together with the names it references.
type MatchInfo struct {
	Match   model.Match
	Shooter string
	Club    string
	Team    string
	// Title is the competition name or the title of a quick analysis.
	Title   string
	Decimal bool
}

func newPDF() (*fpdf.Fpdf, func(string) string) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	return pdf, pdf.UnicodeTranslatorFromDescriptor("")
}

func output(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	return buf.Bytes(), err
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// pyFloat formats like Python's str(float), e.g. 10.0, 132.4.
func pyFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// primary returns the bold and the secondary value of a result.
func primary(s model.Shots, decimal bool) (string, string) {
	if decimal {
		return f1(s.Summe()), strconv.Itoa(s.SummeGanz())
	}
	return strconv.Itoa(s.SummeGanz()), f1(s.Summe())
}

func average(s model.Shots, decimal bool) (string, string) {
	n := float64(len(s))
	dec, whole := f2(s.Summe()/n), f2(float64(s.SummeGanz())/n)
	if decimal {
		return dec, whole
	}
	return whole, dec
}

// MatchPDF renders the shooter report of a match.
func MatchPDF(info MatchInfo) ([]byte, error) {
	m := info.Match
	if len(m.Shots) == 0 {
		return nil, fmt.Errorf("Ergebnis enthält keine Schüsse")
	}
	g, ok := model.Targets[m.TypeOfTarget]
	if !ok {
		return nil, fmt.Errorf("unbekannter Scheibentyp %q", m.TypeOfTarget)
	}
	pdf, tr := newPDF()
	text := func(x, y float64, s string) { pdf.Text(x, y, tr(s)) }
	bold := func(on bool) {
		style := ""
		if on {
			style = "B"
		}
		pdf.SetFont("Helvetica", style, 10)
	}

	pdf.SetFont("Helvetica", "B", 16)
	text(10, 20, info.Title+" "+m.Date)
	drawShotImage(pdf, 210-10-70, 10, 70, g, m.Shots)

	bold(false)
	for _, l := range []struct {
		x, y  float64
		label string
	}{{10, 30, "Name:"}, {10, 35, "Verein:"}, {10, 40, "Mannschaft:"}, {10, 50, "Ergebnis:"},
		{10, 60, "Schnitt:"}, {10, 70, "Bester Teiler:"}, {50, 50, "Ablage R/L:"}, {50, 55, "Ablage H/T:"}} {
		text(l.x, l.y, l.label)
	}
	for i, ring := range []int{10, 9, 8, 7, 6, 5} {
		x := 50 + float64(i)*10
		text(x, 65, fmt.Sprintf("%der", ring))
		text(x, 70, strconv.Itoa(m.Shots.CountRing(ring)))
	}
	text(35, 30, info.Shooter)
	text(35, 35, info.Club)
	text(35, 40, info.Team)

	result, result2 := primary(m.Shots, info.Decimal)
	avg, avg2 := average(m.Shots, info.Decimal)
	bold(true)
	text(35, 50, result)
	text(35, 60, avg)
	text(35, 70, f1(m.Shots.Best().Teiler))
	bold(false)
	text(35, 55, result2)
	text(35, 65, avg2)
	text(75, 50, f1(m.Shots.AblageRL()))
	text(75, 55, f1(m.Shots.AblageHT()))

	const rowHeight = 35
	series := m.Shots.Series()
	row, top, perPage := 0, 80.0, 6
	for k, s := range series {
		if row == perPage {
			pdf.AddPage()
			row, top, perPage = 0, 10, 8
		}
		y0 := top + float64(row)*rowHeight
		row++
		drawShotImage(pdf, 210-10-rowHeight, y0, rowHeight, g, s)
		pdf.SetLineWidth(0.2)
		pdf.Line(10, y0, 200, y0)

		for j, shot := range s {
			x := 11.5*float64(j) + 50
			whole, dec := strconv.Itoa(shot.RingeGanz()), pyFloat(shot.Ringe)
			first, second := whole, dec
			arrowY := y0 + 2
			if info.Decimal {
				first, second = dec, whole
				arrowY += 5
			}
			bold(true)
			text(x, y0+5, first)
			bold(false)
			text(x, y0+10, second)
			arrowX := x + 2
			if shot.Ringe >= 10 {
				arrowX += 2
			}
			drawArrow(pdf, arrowX, arrowY, 4, shot, g)
		}

		text(10, y0+5, fmt.Sprintf("Serie %d", k+1))
		text(10, y0+15, "Ergebnis:")
		text(10, y0+25, "Bester Teiler:")
		text(50, y0+20, "Ablage R/L:")
		text(50, y0+25, "Ablage H/T:")
		result, result2 := primary(s, info.Decimal)
		bold(true)
		text(35, y0+15, result)
		bold(false)
		text(35, y0+20, result2)
		text(35, y0+25, f1(s.Best().Teiler))
		text(70, y0+20, f1(s.AblageRL()))
		text(70, y0+25, f1(s.AblageHT()))

		// Intermediate result after every 40 shots if more follow.
		if (k+1)%4 == 0 && k+1 < len(series) {
			sofar := m.Shots[:(k+1)*10]
			sum, _ := primary(sofar, info.Decimal)
			text(90, y0+20, "Zwischenergebnis:")
			text(90, y0+25, "Schnitt:")
			text(130, y0+20, sum)
			text(130, y0+25, f2(sofar.Result(info.Decimal)/float64(len(sofar))))
		}
	}
	return output(pdf)
}

// RankedEntry is one line of a competition result list.
type RankedEntry struct {
	Shooter string
	Match   model.Match
}

// FormatRankValue formats the value an entry is ranked by.
func FormatRankValue(modus string, shots model.Shots) string {
	v := model.RankValue(modus, shots)
	switch modus {
	case model.ModusBestDecimal, model.ModusBestTeiler:
		return f1(v)
	}
	return strconv.Itoa(int(v))
}

// CompetitionPDF renders the result list. For league competitions home
// and guest entries are listed separately.
func CompetitionPDF(c model.Competition, ranked []RankedEntry, home, guest []RankedEntry) ([]byte, error) {
	pdf, tr := newPDF()
	text := func(x, y float64, s string) { pdf.Text(x, y, tr(s)) }
	pdf.SetFont("Helvetica", "B", 16)
	text(10, 20, fmt.Sprintf("Ergebnisliste %s %s", c.Name, c.Date))
	pdf.SetFont("Helvetica", "", 10)

	y := 30.0
	nextLine := func() {
		y += 5
		if y > 285 {
			pdf.AddPage()
			y = 20
		}
	}
	list := func(entries []RankedEntry, value func(model.Match) string, withSeries bool) {
		for n, e := range entries {
			text(10, y, fmt.Sprintf("%d.", n+1))
			text(15, y, e.Shooter)
			if withSeries {
				for j, s := range e.Match.Shots.Series() {
					result, _ := primary(s, c.Decimal)
					text(70+float64(j)*10, y, result)
				}
			}
			text(190, y, value(e.Match))
			nextLine()
		}
	}

	if c.Modus == model.ModusLeague {
		total := func(m model.Match) string { return strconv.Itoa(m.Shots.SummeGanz()) }
		for _, side := range []struct {
			title   string
			entries []RankedEntry
		}{{"Heim", home}, {"Gast", guest}} {
			pdf.SetFont("Helvetica", "B", 12)
			text(10, y, side.title)
			pdf.SetFont("Helvetica", "", 10)
			nextLine()
			list(side.entries, total, true)
			nextLine()
		}
	} else {
		withSeries := c.Modus == model.ModusBest || c.Modus == model.ModusBestDecimal
		list(ranked, func(m model.Match) string { return FormatRankValue(c.Modus, m.Shots) }, withSeries)
	}
	return output(pdf)
}

// CSV renders the match in the format read by machine.CSV.
func CSV(m model.Match) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "%s;%d;%s\r", m.Date, len(m.Shots), m.TypeOfTarget)
	for _, s := range m.Shots {
		line := strings.Join([]string{pyFloat(s.Ringe), pyFloat(s.Teiler), pyFloat(s.X), pyFloat(s.Y)}, ";")
		b.WriteString(strings.ReplaceAll(line, ".", ",") + "\r")
	}
	return []byte(b.String())
}

// QRString compresses a match into the format read by machine.QR.
func QRString(m model.Match) string {
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(m.Date, ".", "") + m.TypeOfTarget)
	for _, s := range m.Shots {
		whole := s.RingeGanz()
		tenth := int(math.Round((s.Ringe - float64(whole)) * 10))
		if whole < 1 || whole > 10 {
			continue // not representable
		}
		line := fmt.Sprintf("%c%d%s+%s+%s", 'A'+10-whole, tenth, pyFloat(s.Teiler), pyFloat(s.X), pyFloat(s.Y))
		b.WriteString(strings.ReplaceAll(line, "+-", "-"))
	}
	return b.String()
}

// QRPNG renders QRString as PNG.
func QRPNG(m model.Match) ([]byte, error) {
	return qrcode.Encode(QRString(m), qrcode.Medium, 512)
}

// FileDate converts dd.mm.yyyy to yymmdd as used in download file names.
func FileDate(date string) string {
	if t, err := time.Parse("02.01.2006", date); err == nil {
		return t.Format("060102")
	}
	return SafeName(date)
}

// SafeName makes s usable as a file name.
func SafeName(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "_")
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}
