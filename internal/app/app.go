package app

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

var instance *gtk.Application

// Require asserts that instance is initialized and returns it.
func Require() *gtk.Application {
	if instance == nil {
		panic("app.Init required")
	}
	return instance
}

// Init initializes app.
func Init(id string) *gtk.Application {
	if instance != nil {
		return instance
	}

	instance = gtk.NewApplication(id, 0)
	return instance
}
