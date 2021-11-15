package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"os/exec"
	"os/signal"

	_ "embed"

	"github.com/diamondburned/go-buttplug"
	"github.com/diamondburned/go-buttplug/device"
	"github.com/diamondburned/go-buttplug/intiface"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

type manager struct {
	*device.Manager
	*buttplug.Websocket
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	app := gtk.NewApplication("com.github.diamondburned.intiface-gtk", 0)

	var w *Window
	app.ConnectActivate(func() {
		w = activate(ctx, app)
	})

	if exit := app.Run(os.Args); exit != 0 {
		os.Exit(exit)
	}

	// Block until the event loop is done.
	cancel()
	<-w.done
}

//go:embed style.css
var styleCSS string

func activate(ctx context.Context, app *gtk.Application) *Window {
	w := NewWindow(ctx, app)
	w.StartLoading()
	defer w.Show()

	css := gtk.NewCSSProvider()
	css.LoadFromData(styleCSS)

	gtk.StyleContextAddProviderForDisplay(
		w.Window.Widget.Display(), css, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	return w
}

type Window struct {
	*gtk.ApplicationWindow
	main *mainContent
	done chan struct{}
	ctx  context.Context
	cli  string
}

type mainContent struct {
	*gtk.Overlay
	sidebar *gtk.StackSidebar
	stack   *DevicesStack
}

func NewWindow(ctx context.Context, app *gtk.Application) *Window {
	w := gtk.NewApplicationWindow(app)
	w.AddCSSClass("intiface-window")
	w.SetDefaultSize(300, 500)

	go func() {
		<-ctx.Done()
		glib.IdleAdd(func() { w.Destroy() })
	}()

	ctx, cancel := context.WithCancel(ctx)
	w.ConnectDestroy(cancel)

	cli := "intiface-cli"
	if v := os.Getenv("INTIFACE_CLI"); v != "" {
		cli = v
	}

	return &Window{
		ApplicationWindow: w,
		cli:               cli,
		ctx:               ctx,
	}
}

func (w *Window) Loaded(manager *manager) {
	stack := NewDevicesStack(manager)

	sidebar := gtk.NewStackSidebar()
	sidebar.SetStack(stack.Stack)

	noDevices := gtk.NewLabel("No devices yet.")
	noDevices.SetHExpand(true)
	noDevices.SetVExpand(true)

	sidestack := gtk.NewStack()
	sidestack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	sidestack.SetSizeRequest(250, -1)
	sidestack.AddChild(sidebar)
	sidestack.AddChild(noDevices)

	stack.OnDevice(func() {
		if stack.IsEmpty() {
			sidestack.SetVisibleChild(noDevices)
		} else {
			sidestack.SetVisibleChild(sidebar)
		}
	})
	stack.onDevice()

	revealer := gtk.NewRevealer()
	revealer.SetHAlign(gtk.AlignStart)
	revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideRight)
	revealer.AddCSSClass("side-revealer")
	revealer.SetChild(sidestack)

	overlay := gtk.NewOverlay()
	overlay.SetChild(stack)
	overlay.AddOverlay(revealer)
	overlay.SetMeasureOverlay(revealer, true)

	reveal := gtk.NewToggleButton()
	reveal.SetIconName("phone-symbolic")
	reveal.ConnectClicked(func() {
		active := reveal.Active()
		revealer.SetRevealChild(active)

		if active {
			overlay.AddCSSClass("sidebar-open")
		} else {
			overlay.RemoveCSSClass("sidebar-open")
		}
	})

	header := gtk.NewHeaderBar()
	header.PackStart(reveal)

	w.SetChild(overlay)
	w.SetTitlebar(header)
	w.SetSizeRequest(250, -1)
	w.SetDefaultSize(350, 550)

	stack.Connect("notify::visible-child", func() {
		device := stack.VisibleDevice()
		if device != nil {
			w.SetTitle(string(device.Controller.Name) + " ⁠— Intiface")
		}
	})

	reveal.SetActive(true)
	revealer.SetRevealChild(true)

	w.main = &mainContent{
		Overlay: overlay,
		sidebar: sidebar,
		stack:   stack,
	}

	w.SetChild(w.main)
	w.SetTitle("Select a Device ⁠— Intiface")
}

func (w *Window) PromptCLI(err error) {
	error := gtk.NewLabel("")
	error.SetVAlign(gtk.AlignStart)
	error.SetXAlign(0)
	error.SetWrapMode(pango.WrapWordChar)
	error.SetWrap(true)
	error.AddCSSClass("error-label")
	error.SetMarkup(fmt.Sprintf(
		`<span color="red"><b>Error:</b></span> %s`,
		html.EscapeString(err.Error()),
	))

	load := func() {
		w.StartLoading()
	}

	entry := gtk.NewEntry()
	entry.SetText(w.cli)
	entry.ConnectChanged(func() { w.cli = entry.Text() })
	entry.ConnectActivate(load)

	label := gtk.NewLabel("Intiface CLI name")
	label.SetXAlign(0)

	retry := gtk.NewButtonWithLabel("Start")
	retry.AddCSSClass("suggested-action")
	retry.ConnectClicked(load)

	form := gtk.NewBox(gtk.OrientationVertical, 0)
	form.AddCSSClass("cli-prompt-form")
	form.SetVExpand(true)
	form.SetVAlign(gtk.AlignCenter)
	form.SetHAlign(gtk.AlignCenter)
	form.Append(label)
	form.Append(entry)
	form.Append(retry)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("cli-prompt")
	box.SetVExpand(true)
	box.Append(error)
	box.Append(form)

	w.SetChild(box)
	w.SetTitle("Intiface Initialization")

	entry.GrabFocus()
}

func (w *Window) StartLoading() {
	loading := gtk.NewSpinner()
	loading.SetHAlign(gtk.AlignCenter)
	loading.SetVAlign(gtk.AlignCenter)
	loading.Start()
	loading.ConnectUnmap(loading.Stop)

	w.SetChild(loading)
	w.SetTitle("Loading Intiface")

	cli := w.cli

	done := make(chan struct{})
	w.done = done

	go func() {
		ctx, cancel := context.WithCancel(w.ctx)
		defer cancel()

		ws := intiface.NewWebsocket(20000, cli)
		devman := device.NewManager()

		var ok bool

		for ev := range devman.ListenPassthrough(ws.Open(ctx)) {
			switch ev := ev.(type) {
			case *buttplug.ServerInfo:
				ok = true
				glib.IdleAdd(func() {
					w.Loaded(&manager{
						Manager:   devman,
						Websocket: ws.Websocket,
					})
				})
				ws.Send(ctx,
					&buttplug.StartScanning{},
					&buttplug.RequestDeviceList{},
				)
			case error:
				log.Println("buttplug error:", ev)

				var execErr *exec.Error
				if !ok && errors.As(ev, &execErr) {
					cancel()
					glib.IdleAdd(func() { w.PromptCLI(ev) })
				}
			}
		}

		// Event loop will break out after this.
		log.Println("event loop exited")

		close(done)
	}()
}
