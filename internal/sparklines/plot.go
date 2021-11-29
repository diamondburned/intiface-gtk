// Package sparklines provides plotting widgets to draw sparklines.
package sparklines

import (
	"image/color"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/vgcairo"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Plot defines a sparklines widget that can plot.
type Plot struct {
	*gtk.DrawingArea
	*plot.Plot

	needle *plotter.Line // [0]: min, [1]: max
	lines  []*Line

	padding [2]float64 // x, y
	tneedle time.Duration
	trange  time.Duration

	clockHandle glib.SourceHandle
}

// NewPlot creates a new plot.
func NewPlot() *Plot {
	p := &Plot{
		Plot: plot.New(),
	}
	p.SetPadding(0, 0)
	p.HideAxes()
	p.BackgroundColor = color.Transparent

	p.DrawingArea = gtk.NewDrawingArea()
	p.DrawingArea.AddCSSClass("sparklines")
	p.DrawingArea.SetDrawFunc(func(_ *gtk.DrawingArea, t *cairo.Context, x, y int) {
		xf := float64(x)
		yf := float64(y)

		// canvas := draw.NewCanvas(vgcairo.NewCanvas(t), 0, 0)
		// // X is left-right.
		// canvas.Min.X = vg.Length(p.padding[0])
		// canvas.Max.X = vg.Length(xf - p.padding[1])
		// // Y is top-bottom
		// canvas.Min.Y = vg.Length(p.padding[2])
		// canvas.Max.Y = vg.Length(yf - p.padding[3])

		p.Plot.Draw(draw.NewCanvas(
			vgcairo.NewCanvas(t),
			vg.Length(xf-p.padding[0]),
			vg.Length(yf-p.padding[1]),
		))
	})

	currentFPS := 0.0

	updateLoop := func(clock *gdk.FrameClock) {
		if fps := clock.FPS(); fps == currentFPS {
			return
		} else {
			currentFPS = fps
		}

		if p.clockHandle > 0 {
			glib.SourceRemove(p.clockHandle)
			p.clockHandle = 0
		}

		ms := uint(1)
		if currentFPS < 1000 {
			ms = 1000 / uint(currentFPS)
		}

		p.clockHandle = glib.TimeoutAdd(ms, func() bool {
			p.QueueDraw()
			return true
		})
	}

	p.AddTickCallback(func(_ gtk.Widgetter, clock gdk.FrameClocker) bool {
		updateLoop(gdk.BaseFrameClock(clock))
		p.InvalidateTime()
		return true
	})

	return p
}

// SetPadding sets the Plot's padding.
func (p *Plot) SetPadding(xPad, yPad float64) {
	p.padding = [2]float64{xPad, yPad}
	p.X.Padding = vg.Length(xPad)
	p.Y.Padding = vg.Length(yPad)
}

// SetMinHeight sets the minimum height of the plot.
func (p *Plot) SetMinHeight(height int) {
	p.SetSizeRequest(-1, height)
}

func (p *Plot) mustColor(clr color.Color) color.Color {
	if clr != nil {
		return clr
	}

	styles := p.DrawingArea.StyleContext()
	c, ok := styles.LookupColor("theme_fg_color")
	if ok {
		return color.RGBA64{
			R: uint16(0xFFFF * c.Red()),
			G: uint16(0xFFFF * c.Green()),
			B: uint16(0xFFFF * c.Blue()),
			A: uint16(0xFFFF * c.Alpha()),
		}
	}

	return color.White
}

// AddLine adds a new line with the given name.
func (p *Plot) AddLine() *Line {
	line := NewLine(p)
	p.lines = append(p.lines, line)
	p.Plot.Add(line)
	return line
}

// SetNeedle sets the needle with the duration starting from the right edge of
// the graph and subtracted by d.
func (p *Plot) SetNeedle(d time.Duration, clr color.Color, width float64) {
	clr = p.mustColor(clr)

	if p.needle == nil {
		p.needle = &plotter.Line{
			XYs: plotter.XYs{
				{X: 0, Y: p.Plot.Y.Min},
				{X: 0, Y: p.Plot.Y.Max},
			},
			StepStyle: plotter.PreStep,
		}
	}

	p.tneedle = d
	p.needle.LineStyle.Color = clr
	p.needle.LineStyle.Width = vg.Length(width)

	p.InvalidateTime()
}

// SetDuration sets the total horizontal points on the plot by deriving it from
// the given duration.
func (p *Plot) SetDuration(d time.Duration) {
	p.trange = d
	p.InvalidateTime()
}

func (p *Plot) invalidateTime() {
	now := time.Now()

	p.Plot.X.Min = timeX(now.Add(-p.trange))
	p.Plot.X.Max = timeX(now)

	if p.needle != nil {
		p.needle.XYs[0].X = timeX(now.Add(-p.tneedle))
		p.needle.XYs[1].X = p.needle.XYs[0].X
	}
}

// InvalidateTime invalidates the plot's time.
func (p *Plot) InvalidateTime() {
	p.invalidateTime()
	p.addLast()
	p.QueueDraw()
}

// SetRange sets the Y range.
func (p *Plot) SetRange(minY, maxY float64) {
	p.Plot.Y.Min = minY
	p.Plot.Y.Max = maxY

	if p.needle != nil {
		p.needle.XYs[0].Y = minY
		p.needle.XYs[1].Y = maxY
	}
}

const secsInUs = float64(time.Second / time.Microsecond)

func timeX(t time.Time) float64 {
	ms := t.UnixMicro()
	return float64(ms) / secsInUs
}

func xTime(x float64) time.Time {
	return time.UnixMicro(int64(x * secsInUs))
}

func (p *Plot) addLast() {
	return

	for _, line := range p.lines {
		if len(line.XYs) == 0 {
			continue
		}

		// Keep the last point on the edge.
		// If we only have 1 point, then clone it.
		// If the last 2 points are not equal, then still append a new point.
		if len(line.XYs) == 1 || line.XYs[len(line.XYs)-1].Y != line.XYs[len(line.XYs)-2].Y {
			pt := line.XYs[len(line.XYs)-1]
			pt.X = p.X.Max

			line.XYs = append(line.XYs, pt)
			continue
		}

		// Keep the last point's X up to date.
		line.XYs[len(line.XYs)-1].X = p.X.Max

		// TODO: keep first (left-most).
	}
}

func (p *Plot) cleanPoints() {
	for _, line := range p.lines {
		cleanPoints(line, p.X.Min, p.X.Max)
	}
}

func cleanPoints(line *Line, minX, maxX float64) {
	// double threshold
	minX -= line.plot.trange.Seconds()

	// seek until we're no longer at a point smaller than our X, but we should
	// keep the last point.
	var pop int
	for pop < len(line.XYs)-1 && line.XYs[pop].X < minX {
		pop++
	}

	// shift leftwards and grab the new length
	n := copy(line.XYs, line.XYs[pop:])
	// shrink slice
	line.XYs = line.XYs[:n]
}
