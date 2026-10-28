package model

// TargetGeometry describes a target type. All values are in 1/100 mm.
type TargetGeometry struct {
	RadiusTen      float64 `json:"radiusTen"`
	RadiusInnerTen float64 `json:"radiusInnerTen"`
	RingIncrement  float64 `json:"ringIncrement"`
	RadiusBlack    float64 `json:"radiusBlack"`
	RadiusCalibre  float64 `json:"radiusCalibre"`
}

// Width is the diameter of the 1 ring.
func (g TargetGeometry) Width() float64 { return 2 * (g.RadiusTen + 9*g.RingIncrement) }

// TargetTypes in display order.
var TargetTypes = []string{"LP", "KK", "LG", "ZS", "LS", "K3", "AB", "AD", "BS"}

var Targets = map[string]TargetGeometry{
	"LP": {575, 250, 800, 2975, 225},
	"KK": {520, 250, 800, 5620, 280},
	"LG": {25, -25, 250, 1520, 225},
	"ZS": {225, 0, 450, 2025, 232}, // actually 232.5
	"LS": {275, 25, 125, 1525, 225},
	"K3": {312, 150, 480, 3372, 280}, // derived from KK 50m, may differ
	"AB": {300, 25, 600, 4500, 300},  // crossbow 30m international
	"AD": {100, 0, 450, 1450, 225},   // no data found
	"BS": {100, 0, 450, 1450, 225},
}
