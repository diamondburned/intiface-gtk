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

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/go-buttplug"
	"github.com/diamondburned/go-buttplug/device"
	"github.com/diamondburned/go-buttplug/intiface"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/intiface-gtk/internal/app"
	"github.com/diamondburned/intiface-gtk/internal/ui"
)

func main() {
	intiface.EnableConsole = true

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	app := app.Init("com.github.diamondburned.intiface-gtk")

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
	adaptive.Init()

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
	*adaptive.Fold
	sidebar *gtk.StackSidebar
	stack   *ui.DeviceStack
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

func (w *Window) Loaded(manager *ui.Manager) {
	stack := ui.NewDeviceStack(manager)
	stack.SetHExpand(true)

	sidebar := gtk.NewStackSidebar()
	sidebar.SetStack(stack.Stack)

	noDevices := gtk.NewLabel("No devices yet.")
	noDevices.SetHExpand(true)
	noDevices.SetVExpand(true)

	sidestack := gtk.NewStack()
	sidestack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	sidestack.AddChild(sidebar)
	sidestack.AddChild(noDevices)

	stack.OnDevice(func() {
		if stack.IsEmpty() {
			sidestack.SetVisibleChild(noDevices)
		} else {
			sidestack.SetVisibleChild(sidebar)
		}
	})
	stack.TriggerOnDevice()

	fold := adaptive.NewFold(gtk.PosLeft)
	fold.SetSideChild(sidestack)
	fold.SetChild(stack)
	fold.SetFoldWidth(200)
	fold.SetFoldThreshold(450)

	reveal := adaptive.NewFoldRevealButton()
	reveal.SetIconName("phone-symbolic")
	reveal.ConnectFold(fold)

	header := gtk.NewHeaderBar()
	header.PackStart(reveal)

	w.SetChild(fold)
	w.SetTitlebar(header)
	w.SetSizeRequest(250, -1)
	w.SetDefaultSize(450, 500)

	stack.Connect("notify::visible-child", func() {
		device := stack.VisibleDevice()
		if device != nil {
			w.SetTitle(string(device.Controller.Name) + " ⁠— Intiface")
		}
	})

	reveal.Button.SetActive(true)
	fold.SetRevealSide(true)

	w.main = &mainContent{
		Fold:    fold,
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
					w.Loaded(&ui.Manager{
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
