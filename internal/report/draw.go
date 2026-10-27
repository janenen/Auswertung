package report

import (
	"math"
	"strconv"

	"github.com/go-pdf/fpdf"

	"github.com/janenen/auswertung/internal/model"
)

// Target geometry is in 1/100 mm, shot coordinates are in mm.

// drawShotImage draws the target cropped to the area containing the shots
// into the square at (x, y) with the given size in mm.
func drawShotImage(pdf *fpdf.Fpdf, x, y, size float64, g model.TargetGeometry, shots model.Shots) {
	var maxX, maxY float64
	for _, s := range shots {
		maxX = math.Max(maxX, math.Abs(s.X*100))
		maxY = math.Max(maxY, math.Abs(s.Y*100))
	}
	half := math.Min(math.Hypot(maxX, maxY)+g.RadiusCalibre, g.Width()/2)
	scale := size / (2 * half)
	cx, cy := x+size/2, y+size/2

	pdf.ClipRect(x, y, size, size, false)
	pdf.SetFillColor(255, 255, 255)
	pdf.Rect(x, y, size, size, "F")
	drawTarget(pdf, cx, cy, scale, g)

	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(8 * scale)
	pdf.SetFillColor(0, 255, 0)
	for _, s := range shots {
		pdf.Circle(cx+s.X*100*scale, cy-s.Y*100*scale, g.RadiusCalibre*scale, "FD")
	}
	pdf.SetLineWidth(10 * scale)
	best := shots.Best()
	pdf.SetFillColor(255, 0, 0)
	pdf.Circle(cx+best.X*100*scale, cy-best.Y*100*scale, g.RadiusCalibre*scale, "FD")
	pdf.SetFillColor(0, 0, 255)
	pdf.Circle(cx+shots.AblageRL()*100*scale, cy-shots.AblageHT()*100*scale, g.RadiusCalibre/3*scale, "FD")
	pdf.ClipEnd()
}

// drawTarget draws rings and ring numbers centered at (cx, cy) with scale
// in mm per 1/100 mm.
func drawTarget(pdf *fpdf.Fpdf, cx, cy, scale float64, g model.TargetGeometry) {
	pdf.SetFillColor(0, 0, 0)
	pdf.Circle(cx, cy, g.RadiusBlack*scale, "F")
	pdf.SetLineWidth(10 * scale)
	color := func(radius float64) int {
		if radius < g.RadiusBlack {
			return 255
		}
		return 0
	}
	if g.RadiusInnerTen > 0 {
		pdf.SetDrawColor(255, 255, 255)
		pdf.Circle(cx, cy, g.RadiusInnerTen*scale, "D")
	}
	for i := range 10 {
		r := g.RadiusTen + float64(i)*g.RingIncrement
		c := color(r)
		pdf.SetDrawColor(c, c, c)
		pdf.Circle(cx, cy, r*scale, "D")
	}
	fontSize := 0.4 * g.RingIncrement * scale / 25.4 * 72 / 0.7 // cap height ≈ 0.7 em
	pdf.SetFont("Helvetica", "", fontSize)
	for ring := 1; ring <= 8; ring++ {
		r := float64(9-ring)*g.RingIncrement + g.RadiusTen + 0.5*g.RingIncrement
		c := color(r)
		pdf.SetTextColor(c, c, c)
		label := strconv.Itoa(ring)
		w := pdf.GetStringWidth(label)
		h := 0.4 * g.RingIncrement * scale
		for _, d := range [][2]float64{{0, 1}, {0, -1}, {1, 0}, {-1, 0}} {
			pdf.Text(cx+d[0]*r*scale-w/2, cy+d[1]*r*scale+h/2, label)
		}
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(0, 0, 0)
}

// drawArrow draws an arrow pointing in the direction of the shot from the
// center, with a circle if the shot is an inner ten.
func drawArrow(pdf *fpdf.Fpdf, x, y, size float64, s model.Shot, g model.TargetGeometry) {
	unit := size / 50
	r := (25 - 4) * unit
	cx, cy := x+size/2, y+size/2
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(2 * unit)
	if math.Hypot(s.X, s.Y)-g.RadiusCalibre/100 < g.RadiusInnerTen/100 {
		pdf.Circle(cx, cy, r, "D")
	}
	if s.X == 0 && s.Y == 0 {
		return
	}
	alpha := math.Atan2(s.Y, s.X)
	dx, dy := math.Cos(alpha), -math.Sin(alpha)
	ex, ey := cx+r*dx, cy+r*dy
	pdf.Line(cx-r*dx, cy-r*dy, ex, ey)
	tip := 0.3 * 2 * r
	for _, a := range []float64{math.Pi / 4, -math.Pi / 4} {
		back := math.Atan2(-dy, -dx) + a
		pdf.Line(ex, ey, ex+tip*math.Cos(back), ey+tip*math.Sin(back))
	}
}
