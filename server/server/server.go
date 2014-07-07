package main

import (
	"melange/app"
	"melange/dispatcher"
	"melange/tracker"
)

// Main Server
type Melange struct {
	App        *app.Server
	Dispatcher *dispatcher.Server
	Tracker    *tracker.Tracker
}

func (m *Melange) Run(port int) error {
	// Create a New Store for Settings
	settings, err := app.CreateStore("settings.db")
	if err != nil {
		return err
	}

	m.App = &app.Server{
		Suffix:  ".127.0.0.1.xip.io:9001",
		Common:  "http://common.melange",
		Plugins: "http://*.plugins.melange",
		// Other Servers
		Dispatcher: m.Dispatcher,
		Tracker:    m.Tracker,
		// Settings
		Settings: settings,
	}
	return m.App.Run(port)
}

func main() {
	mel := &Melange{}
	mel.Run(9001)
}
