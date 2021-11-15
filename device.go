package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/diamondburned/go-buttplug"
	"github.com/diamondburned/go-buttplug/device"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// DevicesStack is a stasck containing devices.
type DevicesStack struct {
	*gtk.Stack
	manager *manager
	devices map[string]*DevicePage

	onDevice func()
}

// NewDevicesStack creates a new devices stack.
func NewDevicesStack(manager *manager) *DevicesStack {
	s := &DevicesStack{
		Stack:   gtk.NewStack(),
		manager: manager,
		devices: map[string]*DevicePage{},
	}
	s.AddCSSClass("devices-stack")
	s.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	s.Connect("notify::visible-child", func() {
		s.VisibleDevice()
	})

	ch := manager.Broadcaster.Listen()
	s.updateDevices()

	go func() {
		for ev := range ch {
			switch ev := ev.(type) {
			case *buttplug.DeviceAdded:
				glib.IdleAdd(func() { s.addDevice(ev) })
			case *buttplug.DeviceRemoved:
				glib.IdleAdd(func() { s.removeDevice(ev.DeviceIndex) })
			}
		}
	}()

	return s
}

// VisibleDevice returns the currently selected DevicePage, or nil if none is
// selected.
func (s *DevicesStack) VisibleDevice() *DevicePage {
	d, ok := s.devices[s.VisibleChildName()]
	if ok {
		d.Load()
		return d
	}
	return nil
}

// IsEmpty returns true if the DevicesStack is empty.
func (s *DevicesStack) IsEmpty() bool {
	return len(s.devices) == 0
}

// OnDevice sets the callback that's invoked everytime a device is added or
// removed.
func (s *DevicesStack) OnDevice(f func()) {
	s.onDevice = f
}

func (s *DevicesStack) updateDevices() {
	for n, device := range s.devices {
		s.Stack.Remove(device)
		delete(s.devices, n)
	}

	for _, device := range s.manager.Devices() {
		ctrl := s.manager.Controller(s.manager, device.Index)
		s.addController(ctrl)
	}
}

func (s *DevicesStack) addDevice(device *buttplug.DeviceAdded) {
	ctrl := s.manager.Controller(s.manager, device.DeviceIndex)
	s.addController(ctrl)
}

func (s *DevicesStack) addController(ctrl *device.Controller) {
	name := fmt.Sprintf("%d", ctrl.Device.Index)

	page := NewDevicePage(ctrl)
	page.SetName(name)

	s.devices[name] = page
	s.AddTitled(page, name, string(ctrl.Device.Name))

	if s.onDevice != nil {
		s.onDevice()
	}
}

func (s *DevicesStack) removeDevice(ix buttplug.DeviceIndex) {
	name := fmt.Sprintf("%d", ix)

	device, ok := s.devices[name]
	if !ok {
		return
	}

	s.Stack.Remove(device)
	delete(s.devices, name)

	if s.onDevice != nil {
		s.onDevice()
	}
}

// DevicePage is a page for a single device.
type DevicePage struct {
	*gtk.ScrolledWindow
	*device.Controller

	battery *indicator
	rssi    *indicator

	loaded bool
}

// NewDevicePage creates a new device page.
func NewDevicePage(ctrl *device.Controller) *DevicePage {
	scroll := gtk.NewScrolledWindow()
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	scroll.AddCSSClass("device-page")

	return &DevicePage{
		ScrolledWindow: scroll,
		Controller:     ctrl.WithAsync(),
	}
}

func (p *DevicePage) Load() {
	if !p.loaded {
		p.load()
	}
}

func (p *DevicePage) load() {
	p.loaded = true

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

			scale := gtk.NewScaleWithRange(gtk.OrientationVertical, 0, 100, steps)
			scale.SetDigits(0)
			scale.SetInverted(true)
			scale.SetVExpand(true)
			scale.SetValue(0)
			scale.SetDrawValue(true)
			scale.SetFormatValueFunc(func(scale *gtk.Scale, value float64) string {
				return fmt.Sprintf("%.0f%%", value)
			})
			scale.ConnectValueChanged(func() {
				ctrl := p.Controller.WithAsync()
				ctrl.Vibrate(map[int]float64{motor: scale.Value() / 100})
			})

			for i := 0.0; i <= steps; i++ {
				scale.AddMark(i/steps*100, gtk.PosRight, "")
			}

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

	p.battery = newIndicator()
	p.rssi = newIndicator()

	indicators := gtk.NewBox(gtk.OrientationHorizontal, 0)
	indicators.AddCSSClass("indicators")
	indicators.SetHAlign(gtk.AlignCenter)
	indicators.Append(p.battery)
	indicators.Append(p.rssi)

	whole := gtk.NewBox(gtk.OrientationVertical, 0)
	whole.AddCSSClass("device-page-box")
	whole.Append(indicators)
	whole.Append(box)

	p.mapUpdate(whole)
	p.ScrolledWindow.SetChild(whole)
}

func (p *DevicePage) mapUpdate(widget gtk.Widgetter) {
	var t glib.SourceHandle

	canRSSI := true
	canBattery := true

	w := gtk.BaseWidget(widget)
	w.ConnectMap(func() {
		t = glib.TimeoutSecondsAdd(1, func() bool {
			go func() {
				var buttplugErr *buttplug.Error

				if canBattery {
					battery, err := p.Controller.Battery()
					glib.IdleAdd(func() {
						p.battery.update(batteryIndication(battery, err == nil))
					})

					if err != nil {
						log.Println("cannot get battery:", err)
						canBattery = !errors.As(err, &buttplugErr)
					}
				}

				if canRSSI {
					rssi, err := p.Controller.RSSILevel()
					glib.IdleAdd(func() {
						p.rssi.update(rssiIndication(rssi, err == nil))
					})

					if err != nil {
						log.Println("cannot get RSSI level:", err)
						canRSSI = !errors.As(err, &buttplugErr)
					}
				}
			}()
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

type indicator struct {
	*gtk.Box
	Image *gtk.Image
	Label *gtk.Label
}

type indication struct {
	Text  string
	Icon  string
	Class string
}

func newIndicator() *indicator {
	image := gtk.NewImage()
	image.SetIconSize(gtk.IconSizeNormal)

	label := gtk.NewLabel("")

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.Append(image)
	box.Append(label)
	box.SetVisible(false)

	return &indicator{
		Box:   box,
		Image: image,
		Label: label,
	}
}

func (i *indicator) update(ind indication) {
	if ind == (indication{}) {
		i.Box.SetVisible(false)
		return
	}

	i.Label.SetText(ind.Text)
	i.Image.SetFromIconName(ind.Icon)
	i.Box.SetVisible(true)
	i.Box.SetCSSClasses([]string{
		"horizontal",
		"indicator",
		ind.Class,
	})
}

func rssiIndication(rssi float64, ok bool) indication {
	if !ok {
		return indication{}
	}

	switch {
	case rssi < -100:
		return indication{"Weak", "network-cellular-signal-weak", "rssi-weak"}
	case rssi < -90:
		return indication{"Medium", "network-cellular-signal-ok", "rssi-ok"}
	case rssi < -80:
		return indication{"Good", "network-cellular-signal-good", "rssi-good"}
	default:
		return indication{"Excellent", "network-cellular-signal-excellent", "rssi-excellent"}
	}
}

func batteryIndication(battery float64, ok bool) indication {
	if !ok {
		return indication{}
	}

	text := fmt.Sprintf("%.0f%%", battery*100)

	switch {
	case battery <= 0.10:
		return indication{text, "battery-empty-symbolic", "battery-empty"}
	case battery <= 0.40:
		return indication{text, "battery-low-symbolic", "battery-low"}
	case battery <= 0.85:
		return indication{text, "battery-good-symbolic", "battery-good"}
	default:
		return indication{text, "battery-full-symbolic", "battery-full"}
	}
}
