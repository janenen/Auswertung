package report

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/model"
)

func testMatch(n int) model.Match {
	r := rand.New(rand.NewPCG(1, 2))
	m := model.Match{Shooter: "u1", TypeOfTarget: "LG", Date: "04.09.2023", ID: "m1"}
	for range n {
		x, y := r.NormFloat64()*1.5, r.NormFloat64()*1.5
		m.Shots = append(m.Shots, model.Shot{Ringe: float64(int(10.9*10-r.Float64()*20)) / 10, Teiler: float64(int(r.Float64()*5000)) / 10, X: float64(int(x*100)) / 100, Y: float64(int(y*100)) / 100})
	}
	return m
}

func TestMatchPDF(t *testing.T) {
	for _, n := range []int{3, 40, 60, 145} {
		for _, decimal := range []bool{false, true} {
			data, err := MatchPDF(MatchInfo{Match: testMatch(n), Shooter: "Jürgen Müller", Title: "Training", Decimal: decimal})
			if err != nil {
				t.Fatalf("n=%d: %v", n, err)
			}
			if !bytes.HasPrefix(data, []byte("%PDF")) {
				t.Fatalf("n=%d: not a PDF", n)
			}
		}
	}
	if dir := os.Getenv("REPORT_OUT"); dir != "" {
		data, _ := MatchPDF(MatchInfo{Match: testMatch(60), Shooter: "Jürgen Müller", Club: "SV Haaren", Team: "1", Title: "Training", Decimal: false})
		os.WriteFile(filepath.Join(dir, "match.pdf"), data, 0o644)
	}
}

func TestCompetitionPDF(t *testing.T) {
	c := model.Competition{Name: "Preisschießen", Date: "04.09.2023", Modus: model.ModusBest, TypeOfTarget: "LG"}
	var entries []RankedEntry
	for range 70 {
		entries = append(entries, RankedEntry{Shooter: "Schütze", Match: testMatch(40)})
	}
	if _, err := CompetitionPDF(c, entries, nil, nil); err != nil {
		t.Fatal(err)
	}
	c.Modus = model.ModusLeague
	if _, err := CompetitionPDF(c, nil, entries[:4], entries[4:8]); err != nil {
		t.Fatal(err)
	}
}

func TestCSVRoundTrip(t *testing.T) {
	m := testMatch(12)
	target, shots, err := machine.ParseCSV(CSV(m))
	if err != nil {
		t.Fatal(err)
	}
	if target != "LG" || len(shots) != 12 || shots[5] != m.Shots[5] {
		t.Errorf("target=%s shot=%+v want %+v", target, shots[5], m.Shots[5])
	}
}

func TestQRRoundTrip(t *testing.T) {
	m := testMatch(40)
	target, shots, err := machine.ParseQR(QRString(m))
	if err != nil {
		t.Fatal(err)
	}
	if target != "LG" || len(shots) != len(m.Shots) {
		t.Fatalf("target=%s n=%d", target, len(shots))
	}
	for i := range shots {
		if shots[i] != m.Shots[i] {
			t.Errorf("shot %d = %+v, want %+v", i, shots[i], m.Shots[i])
		}
	}
	if _, err := QRPNG(m); err != nil {
		t.Error(err)
	}
}

func TestLeagueXLSX(t *testing.T) {
	sheet := LeagueSheet{
		Competition: model.Competition{Date: "31.07.2025", Count: 40},
		League:      model.RSBLeague{Discipline: "LGa", League: "Kreisliga", Group: "1"},
		Home:        "SV A - 1", Guest: "SV B - 2",
		HomeEntries:  []RankedEntry{{Shooter: "Anna", Match: testMatch(40)}},
		GuestEntries: []RankedEntry{{Shooter: "Bert", Match: testMatch(40)}, {Shooter: "Carl", Match: testMatch(30)}},
	}
	if _, err := LeagueXLSX(filepath.Join(t.TempDir(), "missing.xlsx"), sheet); !errors.Is(err, ErrNoLeagueTemplate) {
		t.Errorf("missing template: %v", err)
	}

	template := filepath.Join("..", "..", "extern", "formular_ligaergebnisse_kr_bez_ll.xlsx")
	if _, err := os.Stat(template); err != nil {
		t.Skip("league template not present")
	}
	data, err := LeagueXLSX(template, sheet)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	s := f.GetSheetName(f.GetActiveSheetIndex())
	series := testMatch(40).Shots.Series()
	for cell, want := range map[string]string{
		"E5": "LGa", "R5": "31.07.2025", "Z4": "1", "B8": "SV A - 1", "H10": "40",
		"D12": "Anna", "H12": strconv.Itoa(series[0].SummeGanz()), "K12": strconv.Itoa(series[3].SummeGanz()),
		"R13": "Carl", "V13": strconv.Itoa(series[0].SummeGanz()), "Y13": "",
	} {
		if got, _ := f.GetCellValue(s, cell); got != want {
			t.Errorf("%s = %q, want %q", cell, got, want)
		}
	}
}

func TestFileNames(t *testing.T) {
	if got := FileDate("04.09.2023"); got != "230904" {
		t.Errorf("FileDate = %q", got)
	}
	if got := SafeName(`A/B: "C"`); got != "A_B___C_" {
		t.Errorf("SafeName = %q", got)
	}
}
