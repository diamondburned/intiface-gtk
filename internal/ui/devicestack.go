package ui

import (
	"fmt"
	"log"

	"github.com/diamondburned/go-buttplug"
	"github.com/diamondburned/go-buttplug/device"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// DeviceStack is a stasck containing devices.
type DeviceStack struct {
	*gtk.Stack
	Manager *Manager
	devices map[string]*DevicePage

	onDevice func()
}

// NewDeviceStack creates a new devices stack.
func NewDeviceStack(Manager *Manager) *DeviceStack {
	s := &DeviceStack{
		Stack:   gtk.NewStack(),
		Manager: Manager,
		devices: map[string]*DevicePage{},
	}
	s.AddCSSClass("devices-stack")
	s.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	s.Connect("notify::visible-child", func() {
		s.VisibleDevice()
	})

	ch := Manager.Broadcaster.Listen()
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

// VisibleDevice returns the currently selected Page, or nil if none is
// selected.
func (s *DeviceStack) VisibleDevice() *DevicePage {
	d, ok := s.devices[s.VisibleChildName()]
	if ok {
		d.Load()
		return d
	}
	return nil
}

// IsEmpty returns true if the DeviceStack is empty.
func (s *DeviceStack) IsEmpty() bool {
	return len(s.devices) == 0
}

// OnDevice sets the callback that's invoked everytime a device is added or
// removed.
func (s *DeviceStack) OnDevice(f func()) {
	s.onDevice = f
}

// TriggerOnDevice triggers the OnDevice callback.
func (s *DeviceStack) TriggerOnDevice() {
	s.onDevice()
}

func (s *DeviceStack) updateDevices() {
	for n, device := range s.devices {
		s.Stack.Remove(device)
		delete(s.devices, n)
	}

	for _, device := range s.Manager.Devices() {
		ctrl := s.Manager.Controller(s.Manager, device.Index)
		s.addController(ctrl)
	}
}

func (s *DeviceStack) addDevice(device *buttplug.DeviceAdded) {
	ctrl := s.Manager.Controller(s.Manager, device.DeviceIndex)
	s.addController(ctrl)
}

func (s *DeviceStack) addController(ctrl *device.Controller) {
	name := fmt.Sprintf("%d", ctrl.Device.Index)

	page := NewDevicePage(ctrl)
	page.SetName(name)

	s.devices[name] = page
	s.AddTitled(page, name, string(ctrl.Device.Name))

	if s.onDevice != nil {
		s.onDevice()
	}
}

func (s *DeviceStack) removeDevice(ix buttplug.DeviceIndex) {
	name := fmt.Sprintf("%d", ix)

	device, ok := s.devices[name]
	if !ok {
		log.Println("unknown device", name, "removed")
		return
	}

	s.Stack.Remove(device)
	delete(s.devices, name)

	if s.onDevice != nil {
		s.onDevice()
	}
}
