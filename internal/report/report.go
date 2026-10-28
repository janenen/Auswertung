// Package report generates the CSV, QR and league form exports of matches
// and competitions. Printable reports are rendered by the web frontend.
package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"github.com/janenen/auswertung/internal/model"
)

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// pyFloat formats like Python's str(float), e.g. 10.0, 132.4.
func pyFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// RankedEntry is one line of a result list.
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
