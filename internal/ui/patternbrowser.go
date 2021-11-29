package ui

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/adaptive"
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
	main *adaptive.LoadablePage
	body *patternBrowserBody
}

func NewPatternBrowser(page *DevicePage) *PatternBrowser {
	b := &PatternBrowser{}
	b.main = adaptive.NewLoadablePage()
	b.main.SetLoading()

	b.Dialog = gtk.NewDialogWithFlags(
		"Patterns ⁠— Intiface",
		app.Require().ActiveWindow(),
		gtk.DialogDestroyWithParent|gtk.DialogUseHeaderBar,
	)
	b.Dialog.AddCSSClass("pattern-browser-dialog")
	b.Dialog.SetDefaultSize(350, 500)
	b.Dialog.SetChild(b.main)

	go func() {
		client := api.NewPatternClient(api.NewClientContext(context.TODO()))

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
	patterns []*patternRow
}

func newPatternBrowserBody(page *DevicePage, patterns []api.Pattern) *patternBrowserBody {
	body := &patternBrowserBody{}
	body.listBox = gtk.NewListBox()
	body.listBox.AddCSSClass("pattern-browser-body")
	body.listBox.SetSelectionMode(gtk.SelectionBrowse)
	body.listBox.SetActivateOnSingleClick(true)

	var lastRow *patternRow
	body.listBox.Connect("row-activated", func(row *gtk.ListBoxRow) {
		if lastRow != nil {
			lastRow.reveal.SetRevealChild(false)
		}
		lastRow = body.patterns[row.Index()]
		lastRow.reveal.SetRevealChild(true)
	})

	body.ScrolledWindow = gtk.NewScrolledWindow()
	body.SetHExpand(true)
	body.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	body.SetChild(body.listBox)

	body.patterns = make([]*patternRow, len(patterns))
	for i := range patterns {
		p := newPatternRow(page, &patterns[i])
		body.patterns[i] = p
		body.listBox.Append(p)
	}

	return body
}

type patternRow struct {
	*gtk.ListBoxRow
	main   *gtk.Box
	info   *patternInfo
	reveal *gtk.Revealer
	tryout *patternTryout
	loaded bool
}

func newPatternRow(page *DevicePage, apiPattern *api.Pattern) *patternRow {
	r := &patternRow{}
	r.info = newPatternInfo(page, apiPattern)

	r.reveal = gtk.NewRevealer()
	r.reveal.AddCSSClass("pattern-tryout-revealer")
	r.reveal.SetRevealChild(false)
	r.reveal.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	r.reveal.Connect("notify::reveal-child", func() {
		r.load(apiPattern)
		if !r.reveal.RevealChild() && r.tryout != nil {
			r.tryout.pause()
		}
	})

	r.main = gtk.NewBox(gtk.OrientationVertical, 0)
	r.main.Append(r.info)
	r.main.Append(r.reveal)

	r.ListBoxRow = gtk.NewListBoxRow()
	r.ListBoxRow.SetOverflow(gtk.OverflowHidden)
	r.ListBoxRow.AddCSSClass("pattern-browse-row")
	r.ListBoxRow.SetChild(r.main)

	return r
}

func (r *patternRow) load(apiPattern *api.Pattern) {
	if r.loaded {
		return
	}

	r.loaded = true

	loading := adaptive.NewLoadablePage()
	loading.SetSizeRequest(-1, 50)
	loading.ErrorPage.SetIconName("")
	r.reveal.SetChild(loading)

	go func() {
		log.Printf("chose pattern\n%#v", apiPattern)

		p, err := httpcache.DownloadPattern(context.TODO(), apiPattern)
		if err != nil {
			glib.IdleAdd(func() { loading.SetError(err) })
			return
		}

		glib.IdleAdd(func() {
			r.tryout = newPatternTryout(r.info.page, p)
			r.tryout.SetSizeRequest(-1, 50)
			r.reveal.SetChild(r.tryout)
		})
	}()
}

type patternInfo struct {
	*gtk.Box // vertical
	page     *DevicePage
	pattern  *api.Pattern

	left   *gtk.Box
	author *gtk.Label
	name   *gtk.Label
	meta   *gtk.Label

	right     *gtk.Box
	length    *components.IconLabel
	favorites *components.IconLabel
	playCount *components.IconLabel
}

var authorAttrs = components.PangoAttrs(
	pango.NewAttrScale(0.85),
	pango.NewAttrForegroundAlpha(components.PercentAlpha(0.85)),
)

func newPatternInfo(page *DevicePage, apiPattern *api.Pattern) *patternInfo {
	p := &patternInfo{
		page:    page,
		pattern: apiPattern,
	}

	date := time.UnixMilli(apiPattern.CreatedTime)
	author := fmt.Sprintf("%s on %s", apiPattern.AuthorOrAnon(), date.Format("02 Jan 2006"))

	p.author = gtk.NewLabel(author)
	p.author.SetHExpand(true)
	p.author.SetXAlign(0)
	p.author.SetTooltipText(author)
	p.author.SetEllipsize(pango.EllipsizeMiddle)
	p.author.SetAttributes(authorAttrs)

	p.name = gtk.NewLabel(apiPattern.DecodedName())
	p.name.SetXAlign(0)
	p.name.SetYAlign(0.5)
	p.name.SetVExpand(true)
	p.name.SetTooltipText(p.name.Text())
	p.name.SetEllipsize(pango.EllipsizeEnd)

	meta := fmt.Sprintf("Version %d", apiPattern.Version2)
	if features := apiPattern.Features(); len(features) > 0 {
		meta += "; " + stringifyFeatures(features)
	}

	p.meta = gtk.NewLabel(meta)
	p.meta.SetXAlign(0)
	p.meta.SetAttributes(authorAttrs)
	p.meta.SetTooltipText(date.Format(time.ANSIC))

	p.left = gtk.NewBox(gtk.OrientationVertical, 2)
	p.left.SetHExpand(true)
	p.left.Append(p.author)
	p.left.Append(p.name)
	p.left.Append(p.meta)

	p.length = components.NewIconLabel("alarm-symbolic", apiPattern.Timer, gtk.PosRight)
	p.length.SetTooltipText(fmt.Sprintf("Duration: %s", apiPattern.Timer))
	p.length.Label.SetXAlign(1)
	p.length.Label.SetAttributes(authorAttrs)

	favoritesCount := strconv.Itoa(int(apiPattern.FavoritesCount))
	p.favorites = components.NewIconLabel("emblem-favorite-symbolic", favoritesCount, gtk.PosRight)
	p.favorites.SetTooltipText(fmt.Sprintf("%s favorites", favoritesCount))
	p.favorites.Label.SetXAlign(1)
	p.favorites.Label.SetAttributes(authorAttrs)

	playCount := strconv.Itoa(int(apiPattern.PlayCount))
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

func stringifyFeatures(feats []pattern.Feature) string {
	var b strings.Builder
	for i, feat := range feats {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString(feat.String())
	}
	return b.String()
}

type patternTryout struct {
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

func newPatternTryout(page *DevicePage, p *pattern.Pattern) *patternTryout {
	t := &patternTryout{
		page:    page,
		pattern: p,
		player:  newPatternPlayer(page, p),
	}
	t.player.F = t.tick

	// b.plot = sparklines.NewPlot()
	// b.plot.AddCSSClass("pattern-tryout-sparkline")
	// b.plot.SetDuration(5 * time.Second)
	// b.plot.SetRange(0, 20)
	// b.plot.SetPadding(4, 6)
	// b.plot.SetMinHeight(80)

	// for i := 0; i < len(p.Features); i++ {
	// 	b.lines = append(b.lines, b.plot.AddLine())
	// }

	t.toggle = gtk.NewButtonFromIconName("media-playback-start-symbolic")
	t.toggle.ConnectClicked(t.togglePlay)

	t.seeker = gtk.NewScaleWithRange(
		gtk.OrientationHorizontal,
		0, patternDuration(p).Seconds(), 1,
	)
	t.seeker.SetHExpand(true)
	t.seeker.SetDrawValue(true)
	t.seeker.SetValuePos(gtk.PosRight)
	t.seeker.ConnectValueChanged(t.seekToBar)

	totalDuration := fmtDuration(patternDuration(p))
	t.seeker.SetFormatValueFunc(func(_ *gtk.Scale, secs float64) string {
		return fmtDuration(secsToDuration(secs)) + "/" + totalDuration
	})

	t.controls = gtk.NewBox(gtk.OrientationHorizontal, 4)
	t.controls.SetHExpand(true)
	t.controls.SetVAlign(gtk.AlignCenter)
	t.controls.Append(t.toggle)
	t.controls.Append(t.seeker)

	t.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	t.Box.AddCSSClass("pattern-tryout-body")
	// b.Box.Append(b.plot)
	t.Box.Append(t.controls)

	return t
}

func secsToDuration(secs float64) time.Duration {
	return time.Duration(secs * float64(time.Second))
}

func (t *patternTryout) togglePlay() {
	if t.player.IsStarted() {
		t.pause()
	} else {
		t.play()
	}
}

func (t *patternTryout) pause() {
	t.player.Stop()
	t.page.stopAll()
	t.RemoveCSSClass("pattern-playing")
	t.toggle.SetIconName("media-playback-start-symbolic")
}

func (t *patternTryout) play() {
	t.player.Start()
	t.AddCSSClass("pattern-playing")
	t.toggle.SetIconName("media-playback-pause-symbolic")
}

func (t *patternTryout) seekToBar() {
	if t.updating {
		return
	}

	t.player.Frame = int(math.Floor(
		// Scale the seeker value (in seconds) down, then scale it up according
		// to the number of points. That gives us the new frame index.
		t.seeker.Value() / patternDuration(t.pattern).Seconds() * float64(len(t.pattern.Points)),
	))

	// Don't call the tick callback. Let the background routine tick by itself,
	// so the point-per-minute stays consistent.
}

func (t *patternTryout) tick() {
	t.player.tick()
	t.updating = true
	t.seeker.SetValue(t.player.CurrentDuration().Seconds())
	t.updating = false
}
