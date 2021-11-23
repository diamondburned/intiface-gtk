package components

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// FailableContent provides a stack that can shift between loading, loaded and
// erroneous.
type FailableContent struct {
	context.Context

	*gtk.Stack
	spinner   *gtk.Spinner
	errorPage *ErrorPage
	content   *gtk.Box
	lastBody  gtk.Widgetter
}

// NewFailableContent creates a new FailableContent widget.
func NewFailableContent() *FailableContent {
	c := &FailableContent{}
	c.spinner = gtk.NewSpinner()
	c.spinner.SetVExpand(true)
	c.spinner.SetHExpand(true)
	c.spinner.SetVAlign(gtk.AlignCenter)
	c.spinner.SetHAlign(gtk.AlignCenter)
	c.spinner.SetSizeRequest(24, 24)
	c.spinner.Start()

	c.errorPage = NewErrorPage()

	c.content = gtk.NewBox(gtk.OrientationHorizontal, 0)
	c.content.SetVExpand(true)
	c.content.SetHExpand(true)

	c.Stack = gtk.NewStack()
	c.Stack.AddChild(c.spinner)
	c.Stack.AddChild(c.errorPage)
	c.Stack.AddChild(c.content)
	c.Stack.SetVisibleChild(c.spinner)
	c.Stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	ctx, cancel := context.WithCancel(context.Background())
	c.Stack.ConnectDestroy(cancel)
	c.Context = ctx

	return c
}

func (c *FailableContent) SetError(err error) {
	c.clearContent()
	c.spinner.Stop()
	c.errorPage.SetError(err)
	c.SetVisibleChild(c.errorPage)
}

func (c *FailableContent) SetLoading() {
	c.clearContent()
	c.spinner.Start()
	c.SetVisibleChild(c.spinner)
}

func (c *FailableContent) SetChild(child gtk.Widgetter) {
	c.clearContent()
	c.spinner.Stop()
	c.lastBody = child
	c.content.Append(child)
	c.SetVisibleChild(c.content)
}

func (c *FailableContent) clearContent() {
	if c.lastBody != nil {
		c.content.Remove(c.lastBody)
	}
}

// ErrorPage is a page showing an error.
type ErrorPage struct {
	*gtk.Box
	header *gtk.Label
	error  *gtk.Label
}

var errorHeaderAttrs = PangoAttrs(
	pango.NewAttrScale(1.15),
)

var errorLabelAttrs = PangoAttrs(
	pango.NewAttrScale(0.9),
	pango.NewAttrForegroundAlpha(PercentAlpha(85)),
)

// NewErrorPage creates a new empty error page.
func NewErrorPage() *ErrorPage {
	// TODO: use an expander to hide most of the error.
	p := &ErrorPage{}
	p.header = gtk.NewLabel("Error")
	p.header.SetAttributes(errorHeaderAttrs)

	p.error = gtk.NewLabel("")
	p.error.SetAttributes(errorLabelAttrs)
	p.error.SetWrap(true)
	p.error.SetWrapMode(pango.WrapWordChar)

	p.Box = gtk.NewBox(gtk.OrientationVertical, 5)
	p.Box.AddCSSClass("error-page")
	p.Box.SetVExpand(true)
	p.Box.SetHExpand(true)
	p.Box.SetVAlign(gtk.AlignCenter)
	p.Box.SetHAlign(gtk.AlignCenter)
	p.Box.Append(p.header)
	p.Box.Append(p.error)

	return p
}

func (p *ErrorPage) SetXAlign(align float32) {
	p.header.SetXAlign(align)
	p.error.SetXAlign(align)
}

// SetError sets the error.
func (p *ErrorPage) SetError(err error) {
	p.error.SetText(err.Error())
}
