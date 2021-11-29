package sparklines

import (
	"image/color"
	"sort"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Line describes a line in a plot.
type Line struct {
	plot *Plot
	// last  vg.Point
	// first vg.Point
	last  plotter.XY
	first plotter.XY
	// XYs is a copy of the points for this line.
	plotter.XYs
	// LineStyle is the style of the line connecting the points.
	// Use zero width to disable lines.
	draw.LineStyle
	// Smooth, if true, will draw the lines smoothly.
	Smooth bool
}

// NewLine creates a new line.
func NewLine(p *Plot) *Line {
	return &Line{
		LineStyle: draw.LineStyle{
			Color: color.Black,
			Width: 1,
		},
		XYs:    make(plotter.XYs, 0, 128), // ~2KB
		first:  plotter.XY{X: p.X.Min, Y: p.Y.Max},
		last:   plotter.XY{X: p.X.Max, Y: p.Y.Max},
		plot:   p,
		Smooth: true,
	}
}

func (l *Line) interpolatePrev() {
	const threshold = 0.100 // ms

	prev := l.last
	if len(l.XYs) > 0 {
		prev = l.XYs[len(l.XYs)-1]
		if prev.X >= (l.plot.X.Max - threshold) {
			return
		}
	}

	// If the last point is far away, then we can craft a fake point in by
	// copying the point subtracted by a few milliseconds.
	prev.X = l.plot.X.Max - threshold
	l.XYs = append(l.XYs, prev)
}

// AddPoint adds the given point into the line.
func (l *Line) AddPoint(pt float64) {
	l.plot.invalidateTime()
	l.interpolatePrev()

	l.XYs = append(l.XYs, plotter.XY{
		X: l.plot.X.Max,
		Y: l.plot.Y.Max - pt,
	})

	l.plot.cleanPoints()
	l.plot.QueueDraw()
}

// SetPoints overrides a line's existing points with the given pts list.
func (l *Line) SetPoints(lineName string, pts map[time.Time]float64) {
	l.plot.invalidateTime()

	minT := xTime(l.plot.Plot.X.Min)

	l.XYs = l.XYs[:0]

	for t, y := range pts {
		if t.Before(minT) {
			continue
		}
		l.XYs = append(l.XYs, plotter.XY{
			X: timeX(t),
			Y: y,
		})
	}

	sort.Slice(l.XYs, func(i, j int) bool {
		return l.XYs[i].X < l.XYs[j].X
	})

	l.plot.QueueDraw()
}

// SetColor sets the line's color.
func (l *Line) SetColor(clr color.Color) {
	l.LineStyle.Color = l.plot.mustColor(clr)
}

// SetWidth sets the line's width or thickness.
func (l *Line) SetWidth(w float64) {
	l.LineStyle.Width = vg.Length(w)
}

func transformPt(pt plotter.XY, trX, trY func(float64) font.Length) vg.Point {
	return vg.Point{X: trX(pt.X), Y: trY(pt.Y)}
}

// Plot plots the line.
func (l *Line) Plot(canvas draw.Canvas, plot *plot.Plot) {
	// updateFirst is true if we have a point that's outside the range.
	// updateFirst := len(l.XYs) > 1 && l.XYs[0].X <= plot.X.Min

	l.first.X = plot.X.Min - l.plot.trange.Seconds()
	l.last.X = plot.X.Max

	// Save the new endpoints if they're there.
	if len(l.XYs) > 0 {
		l.first.Y = l.XYs[0].Y
		l.last.Y = l.XYs[len(l.XYs)-1].Y
	}

	// TODO: gonum/interp

	trX, trY := plot.Transforms(&canvas)

	pts := make([]vg.Point, len(l.XYs)+2)
	for i, p := range l.XYs {
		pts[i+1] = transformPt(p, trX, trY)
	}

	// Ensure the 2 endpoints.
	pts[0] = transformPt(l.first, trX, trY)
	pts[len(pts)-1] = transformPt(l.last, trX, trY)

	var pa vg.Path
	var i int

	if !l.Smooth {
		pa.Move(vg.Point{
			X: vg.Length(pts[i].X),
			Y: vg.Length(pts[i].Y),
		})
		for i = 1; i < len(pts); i++ {
			pa.Line(vg.Point{
				X: vg.Length(pts[i].X),
				Y: vg.Length(pts[i].Y),
			})
		}
	} else {
		pa.Move(vg.Point{
			X: vg.Length(pts[i].X),
			Y: vg.Length(pts[i].Y),
		})
		for i = 1; i < len(pts)-1; i++ {
			xc := (pts[i].X + pts[i+1].X) / 2
			yc := (pts[i].Y + pts[i+1].Y) / 2

			pa.QuadTo(
				vg.Point{X: vg.Length(pts[i].X), Y: vg.Length(pts[i].Y)},
				vg.Point{X: vg.Length(xc), Y: vg.Length(yc)},
			)
		}
		pa.Line(vg.Point{
			X: vg.Length(pts[i].X),
			Y: vg.Length(pts[i].Y),
		})
	}

	canvas.SetLineStyle(l.LineStyle)
	canvas.Stroke(pa)
}
