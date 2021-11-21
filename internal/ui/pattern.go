package ui

import (
	"fmt"
	"html"
	"os"
	"time"

	"github.com/diamondburned/go-lovense/pattern"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/intiface-gtk/internal/app"
	"github.com/diamondburned/intiface-gtk/internal/gticker"
	"github.com/pkg/errors"
)

type patternBox struct {
	*gtk.Frame
	page *DevicePage

	stack   *gtk.Stack
	loadBox *gtk.Box
	loadErr *gtk.Label

	currBox *gtk.Box
	current *patternState
}

func newPatternBox(page *DevicePage) *patternBox {
	frame := gtk.NewFrame("Pattern")
	frame.AddCSSClass("more-pattern")
	frame.SetLabelAlign(0)

	loadErr := gtk.NewLabel("")
	loadErr.SetXAlign(0)
	loadErr.SetWrap(true)
	loadErr.SetWrapMode(pango.WrapWordChar)
	loadErr.SetVisible(false)
	loadErr.AddCSSClass("pattern-error")

	loadBtn := gtk.NewButtonWithLabel("Load File")

	loadBox := gtk.NewBox(gtk.OrientationVertical, 0)
	loadBox.AddCSSClass("pattern-loadfile")
	loadBox.Append(loadErr)
	loadBox.Append(loadBtn)

	currBox := gtk.NewBox(gtk.OrientationVertical, 0)

	stack := gtk.NewStack()
	stack.AddChild(currBox)
	stack.AddChild(loadBox)
	stack.SetVisibleChild(loadBox)
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	b := &patternBox{
		Frame:   frame,
		page:    page,
		stack:   stack,
		loadErr: loadErr,
		loadBox: loadBox,
		currBox: currBox,
	}

	frame.SetChild(stack)
	loadBtn.ConnectClicked(b.loadPattern)

	return b
}

func (b *patternBox) loadPattern() {
	b.stop()

	chooser := gtk.NewFileChooserNative(
		"Load a Pattern",
		app.Require().ActiveWindow(),
		gtk.FileChooserActionOpen,
		"Load", "Cancel",
	)
	chooser.SetModal(true)
	defer chooser.Show()

	chooser.ConnectResponse(func(respID int) {
		if respID != int(gtk.ResponseAccept) {
			return
		}

		file := chooser.File()
		path := file.Path()
		if path == "" {
			b.setLoadErr("chosen file is not local")
			return
		}

		b.open(path)
	})
}

func (b *patternBox) open(path string) {
	b.SetSensitive(false)

	go func() {
		onErr := func(err error) {
			glib.IdleAdd(func() {
				b.setLoadErr(err.Error())
				b.SetSensitive(true)
			})
		}

		f, err := os.Open(path)
		if err != nil {
			onErr(err)
			return
		}
		defer f.Close()

		p, err := pattern.Parse(f)
		if err != nil {
			onErr(errors.Wrap(err, "pattern error"))
			return
		}

		glib.IdleAdd(func() {
			b.setPattern(p)
			b.SetSensitive(true)
		})
	}()
}

func (b *patternBox) setPattern(p *pattern.Pattern) {
	b.current = newPatternState(b.page, p, b.stop)
	b.currBox.Append(b.current)

	b.stack.SetVisibleChild(b.currBox)
	b.Frame.AddCSSClass("pattern-loaded")

	b.loadBox.SetSensitive(false)
}

func (b *patternBox) setLoadErr(err string) {
	b.loadErr.SetMarkup(fmt.Sprintf(
		`<span color="red"><b>Error:</b></span> %s`,
		html.EscapeString(err),
	))
	b.loadErr.SetVisible(true)
}

func (b *patternBox) stop() {
	b.loadErr.SetVisible(false)
	b.loadErr.SetText("")

	if b.current == nil {
		return
	}

	b.loadBox.SetSensitive(true)

	b.stack.SetVisibleChild(b.loadBox)
	b.Frame.RemoveCSSClass("pattern-loaded")

	b.currBox.Remove(b.current)
	b.current.detach()
	b.current = nil
}

type patternState struct {
	*gtk.Box
	pattern *pattern.Pattern
	page    *DevicePage

	playDuration string
	duration     *gtk.Label

	frame  int
	tickFn gticker.Func
}

func newPatternState(page *DevicePage, p *pattern.Pattern, stopFn func()) *patternState {
	s := &patternState{
		pattern: p,
		page:    page,
	}
	s.playDuration = fmtDuration(p.Interval * time.Duration(len(p.Points)))
	s.tickFn.D = p.Interval
	s.tickFn.F = s.tick

	stop := gtk.NewButtonFromIconName("media-playback-stop-symbolic")
	stop.ConnectClicked(stopFn)

	togglePlay := gtk.NewButtonFromIconName("media-playback-start-symbolic")
	togglePlay.ConnectClicked(func() {
		if s.tickFn.IsStarted() {
			s.tickFn.Stop()
			s.page.stopAll()
			s.RemoveCSSClass("pattern-playing")
			togglePlay.SetIconName("media-playback-start-symbolic")
		} else {
			s.tickFn.Start()
			s.AddCSSClass("pattern-playing")
			togglePlay.SetIconName("media-playback-pause-symbolic")
		}
	})

	controls := gtk.NewBox(gtk.OrientationHorizontal, 0)
	controls.AddCSSClass("pattern-controls")
	controls.Append(togglePlay)
	controls.Append(stop)

	s.duration = gtk.NewLabel(s.playDuration)
	s.duration.SetXAlign(0)
	s.duration.SetHExpand(true)

	s.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	s.Box.Append(s.duration)
	s.Box.Append(controls)

	return s
}

func (s *patternState) tick() {
	defer func() {
		s.frame++
		if s.frame > len(s.pattern.Points) {
			s.frame = 0
		}
	}()

	ranges := s.page.ranges
	points := s.pattern.Points[s.frame]
	if len(points) == 0 {
		setRanges(ranges, 0)
		return
	}

	if len(ranges) != len(points) {
		setRanges(ranges, points[0].AsFloat()*100)
		return
	}

	for motor, strength := range s.pattern.Points[s.frame] {
		ranges[motor].SetValue(strength.AsFloat() * 100)
	}

	d := s.pattern.Interval * time.Duration(s.frame)
	s.duration.SetMarkup(fmt.Sprintf(
		"<b>%s</b>/%s",
		fmtDuration(d), s.playDuration,
	))
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", d/time.Minute, d%time.Minute/time.Second)
}

func (s *patternState) detach() {
	s.tickFn.Stop()
}
