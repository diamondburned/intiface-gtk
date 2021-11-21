package ui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

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
		return indication{"Weak", "network-wireless-signal-weak", "rssi-weak"}
	case rssi < -90:
		return indication{"Medium", "network-wireless-signal-ok", "rssi-ok"}
	case rssi < -80:
		return indication{"Good", "network-wireless-signal-good", "rssi-good"}
	default:
		return indication{"Excellent", "network-wireless-signal-excellent", "rssi-excellent"}
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
