// Package pijector uses the Chromium devtools protocol to control a Chromium
// instance in debugging mode.
package pijector

import (
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Kiosk is a single Chromium instance to which pijector is connect.
type Kiosk struct {
	sync.RWMutex
	browser *rod.Browser
	current *rod.Page
	addr    string
}

// Address of the Kiosk's Chromium debugger.
func (k *Kiosk) Address() string {
	return k.addr
}

// Show the URL on the Kiosk.
func (k *Kiosk) Show(u string) error {
	k.Lock()
	defer k.Unlock()
	var loadEvent proto.PageLoadEventFired
	if err := k.current.Navigate(u); err != nil {
		return err
	}
	k.current.WaitEvent(&loadEvent)()
	return nil
}

// Screenshot of the current display.
func (k *Kiosk) Screenshot() ([]byte, error) {
	k.RLock()
	defer k.RUnlock()
	ssReq := &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	}
	return k.current.Screenshot(false, ssReq)
}

// Details of a Kiosk's current display.
type Details struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Details of the Kiosk's current display.
func (k *Kiosk) Details() (*Details, error) {
	k.RLock()
	defer k.RUnlock()
	info, err := k.current.Info()
	if err != nil {
		return nil, err
	}
	return &Details{
		URL:   info.URL,
		Title: info.Title,
	}, nil
}

// Dial a Chromium debugger, and take control of it as a Kiosk.
func Dial(address string) (*Kiosk, error) {
	u, err := launcher.ResolveURL(address)
	if err != nil {
		return nil, err
	}

	browser := rod.New().ControlURL(u).DefaultDevice(devices.Clear)
	if err := browser.Connect(); err != nil {
		return nil, err
	}
	pages, err := browser.Pages()
	if err != nil {
		return nil, err
	}
	if _, err := pages[0].Activate(); err != nil {
		return nil, err
	}
	return &Kiosk{
		browser: browser,
		current: pages[0],
		addr:    address,
	}, nil
}
