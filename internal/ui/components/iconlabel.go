package components

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// IconLabel is a box containing an icon on the left or right of label.
type IconLabel struct {
	*gtk.Box
	Icon  *gtk.Image
	Label *gtk.Label
}

// NewIconLabel creates a new IconLabel with the icon being either by the left
// or right of the label.
func NewIconLabel(icon, text string, iconPos gtk.PositionType) *IconLabel {
	image := gtk.NewImageFromIconName(icon)
	image.SetIconSize(gtk.IconSizeNormal)

	label := gtk.NewLabel(text)
	label.SetHExpand(true)
	label.SetEllipsize(pango.EllipsizeEnd)

	box := gtk.NewBox(gtk.OrientationHorizontal, 2)
	box.AddCSSClass("icon-label")

	switch iconPos {
	case gtk.PosLeft:
		label.SetXAlign(1)
		box.Append(image)
		box.Append(label)
	case gtk.PosRight:
		label.SetXAlign(0)
		box.Append(label)
		box.Append(image)
	default:
		log.Panic("invalid iconPos", iconPos)
	}

	return &IconLabel{
		Box:   box,
		Icon:  image,
		Label: label,
	}
}

// IconLabelButton is an icon-label button.
type IconLabelButton struct {
	*gtk.Button
	IconLabel *IconLabel
}

// NewIconLabelButton creates a new icon-label button widget.
func NewIconLabelButton(icon, text string, iconPos gtk.PositionType) *IconLabelButton {
	il := NewIconLabel(icon, text, iconPos)
	il.Label.SetHExpand(false)
	il.Label.SetXAlign(0.5)

	button := gtk.NewButton()
	button.SetChild(il)
	button.AddCSSClass("icon-label-button")

	return &IconLabelButton{
		Button:    button,
		IconLabel: il,
	}
}
