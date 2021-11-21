package ui

import (
	"github.com/diamondburned/go-buttplug"
	"github.com/diamondburned/go-buttplug/device"
)

type Manager struct {
	*device.Manager
	*buttplug.Websocket
}
