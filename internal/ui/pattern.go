package ui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
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
	b := &patternBox{page: page}

	b.Frame = gtk.NewFrame("Pattern")
	b.Frame.AddCSSClass("more-pattern")
	b.Frame.SetLabelAlign(0)

	b.loadErr = gtk.NewLabel("")
	b.loadErr.SetXAlign(0)
	b.loadErr.SetWrap(true)
	b.loadErr.SetWrapMode(pango.WrapWordChar)
	b.loadErr.SetVisible(false)
	b.loadErr.AddCSSClass("pattern-error")

	loadBtn := gtk.NewButtonWithLabel("Open")
	loadBtn.SetHExpand(true)
	loadBtn.ConnectClicked(b.loadPattern)

	browseBtn := gtk.NewButtonWithLabel("Discover")
	browseBtn.SetHExpand(true)
	browseBtn.ConnectClicked(b.browsePatterns)

	actionBox := gtk.NewBox(gtk.OrientationHorizontal, 4)
	actionBox.Append(loadBtn)
	actionBox.Append(browseBtn)

	b.loadBox = gtk.NewBox(gtk.OrientationVertical, 0)
	b.loadBox.AddCSSClass("pattern-loadfile")
	b.loadBox.Append(b.loadErr)
	b.loadBox.Append(actionBox)

	b.currBox = gtk.NewBox(gtk.OrientationVertical, 0)

	b.stack = gtk.NewStack()
	b.stack.AddChild(b.currBox)
	b.stack.AddChild(b.loadBox)
	b.stack.SetVisibleChild(b.loadBox)
	b.stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	b.Frame.SetChild(b.stack)

	return b
}

func (b *patternBox) browsePatterns() {
	browser := NewPatternBrowser(b.page)
	browser.Show()
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

		name := filepath.Base(path)

		glib.IdleAdd(func() {
			b.setPattern(p, name)
			b.SetSensitive(true)
		})
	}()
}

func (b *patternBox) setPattern(p *pattern.Pattern, name string) {
	b.current = newPatternState(b, p, name)
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
	player  *patternPlayer

	duration *gtk.Label
}

func newPatternState(b *patternBox, p *pattern.Pattern, name string) *patternState {
	s := &patternState{
		pattern: p,
		page:    b.page,
		player:  newPatternPlayer(b.page, p),
	}
	s.player.F = s.tick

	stop := gtk.NewButtonFromIconName("media-playback-stop-symbolic")
	stop.ConnectClicked(b.stop)

	togglePlay := gtk.NewButtonFromIconName("media-playback-start-symbolic")
	togglePlay.ConnectClicked(func() {
		if s.player.IsStarted() {
			s.player.Stop()
			s.page.setZeroValues()
			s.RemoveCSSClass("pattern-playing")
			togglePlay.SetIconName("media-playback-start-symbolic")
		} else {
			s.player.Start()
			s.AddCSSClass("pattern-playing")
			togglePlay.SetIconName("media-playback-pause-symbolic")
		}
	})

	controls := gtk.NewBox(gtk.OrientationHorizontal, 0)
	controls.AddCSSClass("pattern-controls")
	controls.Append(togglePlay)
	controls.Append(stop)

	nameLabel := gtk.NewLabel(name)
	nameLabel.SetXAlign(0)
	nameLabel.SetEllipsize(pango.EllipsizeEnd)
	nameLabel.SetTooltipText(name)

	s.duration = gtk.NewLabel(s.player.TotalDuration)
	s.duration.SetXAlign(0)
	s.duration.SetHExpand(true)

	infoBox := gtk.NewBox(gtk.OrientationVertical, 0)
	infoBox.Append(nameLabel)
	infoBox.Append(s.duration)

	s.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	s.Box.Append(infoBox)
	s.Box.Append(controls)

	return s
}

func (s *patternState) tick() {
	s.player.tick()

	s.duration.SetMarkup(fmt.Sprintf(
		"<b>%s</b>/%s",
		fmtDuration(s.player.CurrentDuration()), s.player.TotalDuration,
	))
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", d/time.Minute, d%time.Minute/time.Second)
}

func patternDuration(p *pattern.Pattern) time.Duration {
	return pointDuration(p, len(p.Points))
}

func pointDuration(p *pattern.Pattern, i int) time.Duration {
	return p.Interval * time.Duration(i)
}

func (s *patternState) detach() {
	s.player.Stop()
}

type patternPlayer struct {
	gticker.Func

	pattern *pattern.Pattern
	page    *DevicePage

	TotalDuration string
	Frame         int
}

func newPatternPlayer(page *DevicePage, pattern *pattern.Pattern) *patternPlayer {
	p := &patternPlayer{
		pattern: pattern,
		page:    page,
	}
	p.TotalDuration = fmtDuration(patternDuration(pattern))
	p.D = pattern.Interval
	p.F = p.tick

	return p
}

func (p *patternPlayer) CurrentDuration() time.Duration {
	return pointDuration(p.pattern, p.Frame)
}

func (p *patternPlayer) tick() {
	p.onTick()

	if p.Frame++; p.Frame >= len(p.pattern.Points) {
		p.Frame = 0
	}
}

func (p *patternPlayer) onTick() {
	ranges := p.page.ranges
	points := p.pattern.Points[p.Frame]
	if len(points) == 0 {
		setRanges(ranges, 0)
		return
	}

	if len(ranges) != len(points) {
		setRanges(ranges, points[0].Scale(p.pattern.Version)*100)
		return
	}

	for motor, strength := range p.pattern.Points[p.Frame] {
		ranges[motor].SetValue(strength.Scale(p.pattern.Version) * 100)
	}
}
