package report

import (
	"errors"
	"fmt"
	"os"

	"github.com/xuri/excelize/v2"

	"github.com/janenen/auswertung/internal/model"
)

// LeagueSheet is the data for the RSB league result form.
type LeagueSheet struct {
	Competition model.Competition
	League      model.RSBLeague
	// Home and Guest are "club - team".
	Home, Guest               string
	HomeEntries, GuestEntries []RankedEntry
}

// Cells of formular_ligaergebnisse_kr_bez_ll.xlsx.
const (
	cellLandesligaGruppe  = "Z2"
	cellBezirksligaGruppe = "Z3"
	cellKreisligaGruppe   = "Z4"
	cellDisziplin         = "E5"
	cellDatum             = "R5"
	cellHeim              = "B8"
	cellGast              = "O8"
	cellSchusszahl        = "H10"

	firstShooterRow = 12
	homeNameCol     = 4  // D
	homeLastSeries  = 12 // series are right-aligned ending before column L (total)
	guestNameCol    = 18 // R
	guestLastSeries = 26 // ending before column Z
)

// ErrNoLeagueTemplate is returned when the form template is missing.
var ErrNoLeagueTemplate = errors.New("Liga-Formular nicht gefunden")

// LeagueXLSX fills the league result form template.
func LeagueXLSX(templatePath string, s LeagueSheet) ([]byte, error) {
	if _, err := os.Stat(templatePath); err != nil {
		return nil, fmt.Errorf("%w (%s)", ErrNoLeagueTemplate, templatePath)
	}
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	c := s.Competition
	values := map[string]any{
		cellDatum:      c.Date,
		cellDisziplin:  s.League.Discipline,
		cellHeim:       s.Home,
		cellGast:       s.Guest,
		cellSchusszahl: c.Count,
	}
	switch s.League.League {
	case "Kreisliga":
		values[cellKreisligaGruppe] = s.League.Group
	case "Bezirksliga":
		values[cellBezirksligaGruppe] = s.League.Group
	case "Landesliga":
		values[cellLandesligaGruppe] = s.League.Group
	}
	for cell, v := range values {
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return nil, err
		}
	}

	seriesCount := c.Count / 10
	for _, side := range []struct {
		entries    []RankedEntry
		nameCol    int
		lastSeries int
	}{
		{s.HomeEntries, homeNameCol, homeLastSeries},
		{s.GuestEntries, guestNameCol, guestLastSeries},
	} {
		for n, e := range side.entries {
			row := firstShooterRow + n
			if err := setCell(f, sheet, side.nameCol, row, e.Shooter); err != nil {
				return nil, err
			}
			for m, series := range e.Match.Shots.Series() {
				if m >= seriesCount {
					break
				}
				if err := setCell(f, sheet, side.lastSeries-seriesCount+m, row, series.SummeGanz()); err != nil {
					return nil, err
				}
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func setCell(f *excelize.File, sheet string, col, row int, v any) error {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	return f.SetCellValue(sheet, cell, v)
}
