package sparklines

import (
	"fmt"
	"image/color"
	"sort"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Line describes a line in a plot.
type Line struct {
	plot  *Plot
	last  vg.Point
	first vg.Point
	// XYs is a copy of the points for this line.
	plotter.XYs
	// LineStyle is the style of the line connecting the points.
	// Use zero width to disable lines.
	draw.LineStyle
}

// NewLine creates a new line.
func NewLine(p *Plot) *Line {
	return &Line{
		LineStyle: draw.LineStyle{
			Color: color.Black,
			Width: 1,
		},
		plot: p,
	}
}

// AddPoint adds the given point into the line.
func (l *Line) AddPoint(pt float64) {
	l.plot.invalidateTime()

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

// SetColorHash sets the line's color by hashing str.
func (l *Line) SetColorHash(str ...interface{}) {
	l.LineStyle.Color = HashColor(fmt.Sprint(str...))
}

// SetWidth sets the line's width or thickness.
func (l *Line) SetWidth(w float64) {
	l.LineStyle.Width = vg.Length(w)
}

// Plot plots the line.
func (l *Line) Plot(canvas draw.Canvas, plot *plot.Plot) {
	// TODO: gonum/interp

	trX, trY := plot.Transforms(&canvas)

	pts := make([]vg.Point, len(l.XYs)+2)
	for i, p := range l.XYs {
		pts[i+1].X = trX(p.X)
		pts[i+1].Y = trY(p.Y)
	}

	// updateFirst is true if we have a point that's outside the range.
	updateFirst := len(l.XYs) > 0 && l.XYs[0].X <= plot.X.Min

	// Initialize the endpoints if they've never been initialized before.
	if l.first == (vg.Point{}) {
		l.first = vg.Point{
			X: trX(plot.X.Min),
			Y: trY(plot.Y.Max),
		}
	}
	if l.last == (vg.Point{}) {
		l.last = vg.Point{
			X: trX(plot.X.Max),
			Y: trY(plot.Y.Max),
		}
	}

	// Save the new endpoints if they're there.
	if len(l.XYs) > 0 {
		l.last = pts[len(pts)-2]
		l.last.X = trX(plot.X.Max)
	}
	if updateFirst {
		l.first = pts[1]
	}

	// Ensure the 2 endpoints.
	pts[0] = l.first
	pts[len(pts)-1] = l.last

	var pa vg.Path
	var i int

	pa.Move(vg.Point{
		X: vg.Length(pts[i].X),
		Y: vg.Length(pts[i].Y),
	})

	if len(pts) > 2 {
		for i = 1; i < len(pts)-2; i++ {
			xc := (pts[i].X + pts[i+1].X) / 2
			yc := (pts[i].Y + pts[i+1].Y) / 2

			pa.QuadTo(
				vg.Point{X: vg.Length(pts[i].X), Y: vg.Length(pts[i].Y)},
				vg.Point{X: vg.Length(xc), Y: vg.Length(yc)},
			)
		}
	}

	pa.QuadTo(
		vg.Point{X: vg.Length(pts[i+0].X), Y: vg.Length(pts[i+0].Y)},
		vg.Point{X: vg.Length(pts[i+1].X), Y: vg.Length(pts[i+1].Y)},
	)

	canvas.SetLineStyle(l.LineStyle)
	canvas.Stroke(pa)
}
