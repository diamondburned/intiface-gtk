// Package components contains generic UI components.
package components

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/pango"
)

// PangoAttrs creates an AttrList from the given Attributes.
func PangoAttrs(attrs ...*pango.Attribute) *pango.AttrList {
	list := pango.NewAttrList()
	for _, attr := range attrs {
		list.Insert(attr)
	}
	return list
}

// PercentAlpha converts [0, 100]% to uint16.
func PercentAlpha(percentage float64) uint16 {
	if percentage > 1 {
		percentage /= 100
	}
	return uint16(math.Round(percentage * 0xFFFF))
}
