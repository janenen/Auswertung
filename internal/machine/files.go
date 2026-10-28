package machine

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/janenen/auswertung/internal/model"
)

// CSV reads files written by the CSV export:
//
//	dd.mm.yyyy;count;TT
//	ringe;teiler;x;y   (decimal comma)
type CSV struct{}

func (CSV) ID() string     { return "csv" }
func (CSV) Name() string   { return "CSV aus Datei" }
func (CSV) Input() string  { return InputFile }
func (CSV) Accept() string { return ".csv" }

var csvHeader = regexp.MustCompile(`^(\d{2}.\d{2}.\d{4});(\d+);(?:b')?(\w{2})'?$`)

// ParseCSV returns the target type and shots of a CSV file.
func ParseCSV(data []byte) (string, model.Shots, error) {
	text := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n")
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "", nil, errors.New("CSV-Datei ist leer")
	}
	h := csvHeader.FindStringSubmatch(lines[0])
	if h == nil {
		return "", nil, errors.New("Format des Dateikopfs passt nicht")
	}
	count, _ := strconv.Atoi(h[2])
	var shots model.Shots
	for i, l := range lines[1:] {
		f := strings.Split(strings.ReplaceAll(l, ",", "."), ";")
		if len(f) != 4 {
			return "", nil, fmt.Errorf("Zeile %d: erwartet 4 Werte", i+2)
		}
		var v [4]float64
		for j := range f {
			var err error
			if v[j], err = strconv.ParseFloat(f[j], 64); err != nil {
				return "", nil, fmt.Errorf("Zeile %d: %w", i+2, err)
			}
		}
		shots = append(shots, model.Shot{Ringe: v[0], Teiler: v[1], X: v[2], Y: v[3]})
	}
	if len(shots) > count {
		return "", nil, errors.New("zu viele Schüsse in der CSV-Datei")
	}
	return h[3], shots, nil
}

func (CSV) Read(ctx context.Context, req Request, out chan<- Reading) error {
	targetType, shots, err := ParseCSV(req.Data)
	if err != nil {
		return err
	}
	return sendAll(ctx, out, shots, targetType)
}

// QR reads the compressed result string (see report.QRString):
// ddmmyyyyTT followed by shots of the form <ring letter><tenth><teiler><±x><±y>.
type QR struct{}

func (QR) ID() string     { return "qr" }
func (QR) Name() string   { return "QR-Code aus Eingabe" }
func (QR) Input() string  { return InputText }
func (QR) Accept() string { return "" }

var (
	qrHeader = regexp.MustCompile(`^\d{8}\w{2}`)
	// Teiler and coordinates are integers in old codes and decimals in
	// codes written by the export.
	qrShot = regexp.MustCompile(`([A-J])(\d)(\d+(?:\.\d+)?)([+-]\d+(?:\.\d+)?)([+-]\d+(?:\.\d+)?)`)
)

// ParseQR returns the target type and shots of a QR code string.
func ParseQR(code string) (string, model.Shots, error) {
	code = strings.TrimSpace(code)
	if !qrHeader.MatchString(code) {
		return "", nil, errors.New("Format des QR-Code-Kopfs passt nicht")
	}
	var shots model.Shots
	for _, g := range qrShot.FindAllStringSubmatch(code[10:], -1) {
		ring := float64(10 - (g[1][0] - 'A'))
		tenth, _ := strconv.Atoi(g[2])
		teiler, _ := strconv.ParseFloat(g[3], 64)
		x, _ := strconv.ParseFloat(g[4], 64)
		y, _ := strconv.ParseFloat(g[5], 64)
		shots = append(shots, model.Shot{Ringe: ring + float64(tenth)/10, Teiler: teiler, X: x, Y: y})
	}
	if len(shots) == 0 {
		return "", nil, errors.New("keine Schüsse im QR-Code gefunden")
	}
	return code[8:10], shots, nil
}

func (QR) Read(ctx context.Context, req Request, out chan<- Reading) error {
	targetType, shots, err := ParseQR(string(req.Data))
	if err != nil {
		return err
	}
	return sendAll(ctx, out, shots, targetType)
}

// QSD reads QuickShot files: records of five little-endian float64
// (ringe, x, y, teiler, unused), terminated by a zero record or EOF.
type QSD struct{}

func (QSD) ID() string     { return "qsd" }
func (QSD) Name() string   { return "QSD aus Datei" }
func (QSD) Input() string  { return InputFile }
func (QSD) Accept() string { return ".qsd" }

// ParseQSD returns the shots of a QSD file.
func ParseQSD(data []byte) (model.Shots, error) {
	var shots model.Shots
	r := bytes.NewReader(data)
	for {
		var rec [5]float64
		err := binary.Read(r, binary.LittleEndian, &rec)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if rec == [5]float64{} {
			break
		}
		shots = append(shots, model.Shot{Ringe: math.Round(rec[0]*10) / 10, X: rec[1], Y: rec[2], Teiler: rec[3]})
	}
	if len(shots) == 0 {
		return nil, errors.New("keine Schüsse in der QSD-Datei")
	}
	return shots, nil
}

func (QSD) Read(ctx context.Context, req Request, out chan<- Reading) error {
	shots, err := ParseQSD(req.Data)
	if err != nil {
		return err
	}
	return sendAll(ctx, out, shots, req.TargetType)
}
