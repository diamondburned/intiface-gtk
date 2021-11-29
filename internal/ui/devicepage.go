package ui

import (
	"errors"
	"fmt"
	"time"

	"github.com/diamondburned/go-buttplug"
	"github.com/diamondburned/go-buttplug/device"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/intiface-gtk/internal/sparklines"
)

type valueRange interface {
	Value() float64
	SetValue(float64)
}

func setRanges(ranges []valueRange, v float64) {
	for _, vrange := range ranges {
		vrange.SetValue(v)
	}
}

// DevicePage is a page for a single device.
type DevicePage struct {
	*gtk.Box
	*device.Controller
	ranges []valueRange

	sparklines *sparklines.Plot

	scroll  *gtk.ScrolledWindow
	battery *indicator
	rssi    *indicator

	actions *gtk.ActionBar

	canRSSI    bool
	canBattery bool
	loaded     bool
}

// NewDevicePage creates a new device page.
func NewDevicePage(ctrl *device.Controller) *DevicePage {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("device-page")

	return &DevicePage{
		Box:        box,
		Controller: ctrl.WithAsync(),
		canRSSI:    true,
		canBattery: true,
	}
}

func (p *DevicePage) Load() {
	if !p.loaded {
		p.load()
	}
}

func (p *DevicePage) load() {
	p.loaded = true
	p.loadGraph()
	p.loadBody()
	p.loadBelow()
}

func (p *DevicePage) loadGraph() {
	p.sparklines = sparklines.NewPlot()
	p.sparklines.AddCSSClass("vibrator-sparkline")
	p.sparklines.SetDuration(3 * time.Second)
	p.sparklines.SetRange(0, 100)
	p.sparklines.SetPadding(4, 2)
	p.sparklines.SetMinHeight(80)
	// p.sparklines.SetNeedle(0, color.RGBA{255, 0, 0, 255}, 2)

	p.Box.Append(p.sparklines)
}

func (p *DevicePage) loadBody() {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignFill)
	box.SetVExpand(true)
	box.AddCSSClass("device-controls")

	if motorSteps := p.VibrationSteps(); len(motorSteps) > 0 {
		child := gtk.NewBox(gtk.OrientationHorizontal, 0)

		for motor, steps := range p.VibrationSteps() {
			motor := motor
			steps := float64(steps)

			line := p.sparklines.AddLine()
			line.Smooth = true
			line.SetWidth(2)
			line.SetColorHash("v", 2<<((motor+1)*8)) // make int variance larger

			scale := gtk.NewScaleWithRange(gtk.OrientationVertical, 0, 100, 100/steps)
			scale.SetDigits(2)
			scale.SetInverted(true)
			scale.SetVExpand(true)
			scale.SetValue(0)
			scale.SetDrawValue(true)
			scale.SetFormatValueFunc(func(scale *gtk.Scale, value float64) string {
				return fmt.Sprintf("%.0f%%", value)
			})
			scale.ConnectValueChanged(func() {
				value := scale.Value()
				line.AddPoint(value)
				p.Controller.Vibrate(map[int]float64{motor: value / 100})
			})

			p.ranges = append(p.ranges, scale)

			// for i := 0.0; i <= steps; i++ {
			// 	scale.AddMark(i/steps*100, gtk.PosRight, "")
			// }

			box := gtk.NewBox(gtk.OrientationVertical, 2)
			box.Append(scale)
			box.Append(gtk.NewLabel(fmt.Sprintf("Motor %d", motor)))

			child.Append(box)
		}

		frame := gtk.NewFrame("Vibrator")
		frame.AddCSSClass("vibrators")
		frame.SetLabelAlign(0)
		frame.SetChild(child)

		box.Append(frame)
	}

	p.scroll = gtk.NewScrolledWindow()
	p.scroll.SetHExpand(true)
	p.scroll.SetVExpand(true)
	p.scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	p.scroll.SetChild(box)

	p.Box.Append(p.scroll)
}

func (p *DevicePage) loadBelow() {
	p.battery = newIndicator()
	p.battery.update(indication{
		Text:  "Unknown",
		Icon:  "battery-missing-symbolic",
		Class: "indicator-unknown",
	})

	p.rssi = newIndicator()
	p.rssi.update(indication{
		Text:  "Unknown",
		Icon:  "network-wireless-signal-none-symbolic",
		Class: "indicator-unknown",
	})

	indicators := gtk.NewBox(gtk.OrientationHorizontal, 0)
	indicators.AddCSSClass("indicators")
	indicators.SetHAlign(gtk.AlignCenter)
	indicators.Append(p.battery)
	indicators.Append(p.rssi)

	p.updateIndicators()
	p.mapUpdate(indicators)

	stopAll := gtk.NewButtonFromIconName("system-shutdown-symbolic")
	stopAll.SetTooltipText("Stop All")
	stopAll.ConnectClicked(p.stopAll)

	more := gtk.NewBox(gtk.OrientationVertical, 0)
	more.AddCSSClass("more")
	more.Append(newPatternBox(p))

	moreScroll := gtk.NewScrolledWindow()
	moreScroll.SetChild(more)
	moreScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	moreScroll.SetPropagateNaturalHeight(true)
	moreScroll.SetMaxContentHeight(200)

	reveal := gtk.NewRevealer()
	reveal.SetVExpand(false)
	reveal.SetChild(moreScroll)
	reveal.SetTransitionType(gtk.RevealerTransitionTypeSlideUp)

	revealButton := gtk.NewToggleButton()
	revealButton.SetIconName("open-menu-symbolic")
	revealButton.ConnectClicked(func() {
		active := revealButton.Active()
		reveal.SetRevealChild(active)
	})

	p.actions = gtk.NewActionBar()
	p.actions.SetCenterWidget(revealButton)
	p.actions.PackStart(indicators)
	p.actions.PackEnd(stopAll)

	p.Box.Append(p.actions)
	p.Box.Append(reveal)
}

func (p *DevicePage) updateIndicators() {
	if p.canBattery {
		go func() {
			battery, err := p.Controller.Battery()
			glib.IdleAdd(func() {
				p.battery.update(batteryIndication(battery, err == nil))

				var buttplugErr *buttplug.Error
				if err != nil && errors.As(err, &buttplugErr) {
					p.canBattery = false
				}
			})
		}()
	}

	if p.canRSSI {
		go func() {
			rssi, err := p.Controller.RSSILevel()
			glib.IdleAdd(func() {
				p.rssi.update(rssiIndication(rssi, err == nil))

				var buttplugErr *buttplug.Error
				if err != nil && errors.As(err, &buttplugErr) {
					p.canRSSI = false
				}
			})
		}()
	}
}

func (p *DevicePage) keepAlive() {
	for _, rangeValue := range p.ranges {
		rangeValue.SetValue(rangeValue.Value())
	}
}

const batteryUpdateFreq = 25 // seconds

func (p *DevicePage) mapUpdate(widget gtk.Widgetter) {
	var t glib.SourceHandle

	w := gtk.BaseWidget(widget)

	w.ConnectMap(func() {
		t = glib.TimeoutSecondsAdd(batteryUpdateFreq, func() bool {
			p.updateIndicators()
			p.keepAlive()
			return true
		})
	})

	w.ConnectUnmap(func() {
		if t > 0 {
			glib.SourceRemove(t)
			t = 0
		}
	})
}

func (p *DevicePage) stopAll() {
	setRanges(p.ranges, 0)
}
