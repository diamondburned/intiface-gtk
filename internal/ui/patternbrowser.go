package ui

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/diamondburned/go-lovense/api"
	"github.com/diamondburned/go-lovense/pattern"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/intiface-gtk/internal/app"
	"github.com/diamondburned/intiface-gtk/internal/httpcache"
	"github.com/diamondburned/intiface-gtk/internal/ui/components"
)

type PatternBrowser struct {
	*gtk.Dialog
	main *components.FailableContent
	body *patternBrowserBody
}

func NewPatternBrowser(page *DevicePage) *PatternBrowser {
	window := app.Require().ActiveWindow()

	b := &PatternBrowser{}

	b.main = components.NewFailableContent()
	b.main.SetLoading()

	b.Dialog = gtk.NewDialogWithFlags("Patterns ⁠— Intiface", window, gtk.DialogDestroyWithParent)
	b.Dialog.AddCSSClass("pattern-browser-dialog")
	b.Dialog.SetApplication(app.Require())
	b.Dialog.SetDefaultSize(window.DefaultSize())
	b.Dialog.SetChild(b.main)

	go func() {
		client := api.NewPatternClient(api.NewClientContext(b.main))

		patterns, err := client.Find(1, 50, api.FindRecommendedPatterns)
		if err != nil {
			glib.IdleAdd(func() { b.main.SetError(err) })
			return
		}

		glib.IdleAdd(func() {
			b.body = newPatternBrowserBody(page, patterns)
			b.main.SetChild(b.body)
		})
	}()

	return b
}

type patternBrowserBody struct {
	*gtk.ScrolledWindow
	listBox  *gtk.ListBox
	patterns []*patternPreview
}

func newPatternBrowserBody(page *DevicePage, patterns []api.Pattern) *patternBrowserBody {
	body := &patternBrowserBody{}
	body.listBox = gtk.NewListBox()
	body.listBox.AddCSSClass("pattern-browser-body")
	body.listBox.SetActivateOnSingleClick(true)
	body.listBox.Connect("row-activated", func(row *gtk.ListBoxRow) {
		pattern := body.patterns[row.Index()]
		pattern.tryout()
	})

	body.ScrolledWindow = gtk.NewScrolledWindow()
	body.SetHExpand(true)
	body.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	body.SetChild(body.listBox)

	body.patterns = make([]*patternPreview, len(patterns))
	for i := range patterns {
		p := newPatternPreview(page, &patterns[i])
		body.patterns[i] = p

		r := gtk.NewListBoxRow()
		r.AddCSSClass("pattern-browse-row")
		r.SetChild(p)

		body.listBox.Append(r)
	}

	return body
}

type patternPreview struct {
	*gtk.Box // vertical
	page     *DevicePage
	pattern  *api.Pattern

	left   *gtk.Box
	author *gtk.Label
	name   *gtk.Label
	date   *gtk.Label

	right     *gtk.Box
	length    *components.IconLabel
	favorites *components.IconLabel
	playCount *components.IconLabel
}

var authorAttrs = components.PangoAttrs(
	pango.NewAttrScale(0.85),
	pango.NewAttrForegroundAlpha(components.PercentAlpha(0.85)),
)

func newPatternPreview(page *DevicePage, pattern *api.Pattern) *patternPreview {
	p := &patternPreview{
		page:    page,
		pattern: pattern,
	}

	p.author = gtk.NewLabel(pattern.AuthorOrAnon())
	p.author.SetHExpand(true)
	p.author.SetXAlign(0)
	p.author.SetTooltipText(p.author.Text())
	p.author.SetEllipsize(pango.EllipsizeEnd)
	p.author.SetAttributes(authorAttrs)

	p.name = gtk.NewLabel(pattern.DecodedName())
	p.name.SetXAlign(0)
	p.name.SetYAlign(0.5)
	p.name.SetVExpand(true)
	p.name.SetTooltipText(p.name.Text())
	p.name.SetEllipsize(pango.EllipsizeEnd)

	date := time.UnixMilli(pattern.CreatedTime)
	p.date = gtk.NewLabel(date.Format("02 Jan 2006"))
	p.date.SetXAlign(0)
	p.date.SetAttributes(authorAttrs)
	p.date.SetTooltipText(date.Format(time.ANSIC))

	p.left = gtk.NewBox(gtk.OrientationVertical, 2)
	p.left.SetHExpand(true)
	p.left.Append(p.author)
	p.left.Append(p.name)
	p.left.Append(p.date)

	p.length = components.NewIconLabel("alarm-symbolic", pattern.Timer, gtk.PosRight)
	p.length.SetTooltipText(fmt.Sprintf("Duration: %s", pattern.Timer))
	p.length.Label.SetXAlign(1)
	p.length.Label.SetAttributes(authorAttrs)

	favoritesCount := strconv.Itoa(int(pattern.FavoritesCount))
	p.favorites = components.NewIconLabel("emblem-favorite-symbolic", favoritesCount, gtk.PosRight)
	p.favorites.SetTooltipText(fmt.Sprintf("%s favorites", favoritesCount))
	p.favorites.Label.SetXAlign(1)
	p.favorites.Label.SetAttributes(authorAttrs)

	playCount := strconv.Itoa(int(pattern.PlayCount))
	p.playCount = components.NewIconLabel("folder-download-symbolic", playCount, gtk.PosRight)
	p.playCount.SetTooltipText(fmt.Sprintf("%s plays", playCount))
	p.playCount.Label.SetXAlign(1)
	p.playCount.Label.SetAttributes(authorAttrs)

	p.right = gtk.NewBox(gtk.OrientationVertical, 0)
	p.right.AddCSSClass("pattern-stats")
	p.right.Append(p.length)
	p.right.Append(p.favorites)
	p.right.Append(p.playCount)

	p.favorites.SetVExpand(true) // middle

	p.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	p.Box.AddCSSClass("pattern-browse-item")
	p.Box.Append(p.left)
	p.Box.Append(p.right)

	return p
}

func (p *patternPreview) tryout() {
	tryout := newPatternTryout(p.page, p.pattern)
	tryout.Show()
}

type patternTryout struct {
	*gtk.Dialog
	page *DevicePage
	main *gtk.Box
	body *components.FailableContent
	info *patternPreview
}

func newPatternTryout(page *DevicePage, p *api.Pattern) *patternTryout {
	t := &patternTryout{page: page}

	t.body = components.NewFailableContent()
	t.body.SetVExpand(true)

	t.info = newPatternPreview(page, p)
	t.info.SetVExpand(false)

	t.main = gtk.NewBox(gtk.OrientationVertical, 0)
	t.main.AddCSSClass("pattern-tryout")
	t.main.Append(t.body)
	t.main.Append(t.info)

	t.Dialog = gtk.NewDialogWithFlags(
		p.DecodedName()+" ⁠— Intiface",
		app.Require().ActiveWindow(),
		gtk.DialogModal|gtk.DialogUseHeaderBar|gtk.DialogDestroyWithParent,
	)
	t.Dialog.AddCSSClass("pattern-tryout-dialog")
	t.Dialog.SetApplication(app.Require())
	t.Dialog.SetDefaultSize(400, 100)
	t.Dialog.SetChild(t.main)

	saveAs := t.Dialog.AddButton("Save As", int(gtk.ResponseApply)).(*gtk.Button)
	saveAs.AddCSSClass("suggested-action")
	saveAs.ConnectClicked(t.saveAs)

	go func() {
		log.Printf("URL: %#v", p)

		p, err := httpcache.DownloadPattern(t.body, p)
		if err != nil {
			glib.IdleAdd(func() { t.body.SetError(err) })
			return
		}

		glib.IdleAdd(func() {
			body := newPatternTryoutBody(page, p)
			t.body.SetChild(body)
		})
	}()

	return t
}

func (t *patternTryout) saveAs() {}

type patternTryoutBody struct {
	*gtk.Box
	page    *DevicePage
	pattern *pattern.Pattern

	// plot   *sparklines.Plot
	// lines  []*sparklines.Line
	player *patternPlayer

	controls *gtk.Box
	toggle   *gtk.Button
	seeker   *gtk.Scale

	updating bool
}

func newPatternTryoutBody(page *DevicePage, p *pattern.Pattern) *patternTryoutBody {
	b := &patternTryoutBody{
		page:    page,
		pattern: p,
		player:  newPatternPlayer(page, p),
	}
	b.player.F = b.tick

	// b.plot = sparklines.NewPlot()
	// b.plot.AddCSSClass("pattern-tryout-sparkline")
	// b.plot.SetDuration(5 * time.Second)
	// b.plot.SetRange(0, 20)
	// b.plot.SetPadding(4, 6)
	// b.plot.SetMinHeight(80)

	// for i := 0; i < len(p.Features); i++ {
	// 	b.lines = append(b.lines, b.plot.AddLine())
	// }

	b.toggle = gtk.NewButtonFromIconName("media-playback-start-symbolic")
	b.toggle.ConnectClicked(func() {
		if b.player.IsStarted() {
			b.player.Stop()
			b.page.stopAll()
			b.RemoveCSSClass("pattern-playing")
			b.toggle.SetIconName("media-playback-start-symbolic")
		} else {
			b.player.Start()
			b.AddCSSClass("pattern-playing")
			b.toggle.SetIconName("media-playback-pause-symbolic")
		}
	})

	b.seeker = gtk.NewScaleWithRange(
		gtk.OrientationHorizontal,
		0, patternDuration(p).Seconds(), 1,
	)
	b.seeker.SetHExpand(true)
	b.seeker.SetDrawValue(true)
	b.seeker.SetValuePos(gtk.PosRight)
	b.seeker.ConnectValueChanged(b.seekToBar)

	totalDuration := fmtDuration(patternDuration(p))
	b.seeker.SetFormatValueFunc(func(_ *gtk.Scale, secs float64) string {
		return fmtDuration(secsToDuration(secs)) + "/" + totalDuration
	})

	b.controls = gtk.NewBox(gtk.OrientationHorizontal, 4)
	b.controls.SetHExpand(true)
	b.controls.SetVAlign(gtk.AlignCenter)
	b.controls.Append(b.toggle)
	b.controls.Append(b.seeker)

	b.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.Box.AddCSSClass("pattern-tryout-body")
	// b.Box.Append(b.plot)
	b.Box.Append(b.controls)

	return b
}

func secsToDuration(secs float64) time.Duration {
	s, ns := math.Modf(secs)
	return time.Duration(s)*time.Second + time.Duration(ns*float64(time.Second))
}

func (t *patternTryoutBody) seekToBar() {
	if t.updating {
		return
	}
	// TODO
}

func (t *patternTryoutBody) tick() {
	t.player.tick()
	t.updating = true
	t.seeker.SetValue(t.player.CurrentDuration().Seconds())
	t.updating = false
}
