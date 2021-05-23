// Package pijector uses the Chromium devtools protocol to control a Chromium
// instance in debugging mode.
package pijector

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
)

var (
	pijectorV1Space = uuid.Must(uuid.Parse("ed77ce29-9f8f-4d4c-87c3-91078f49e9f9"))
	localKioskSpace = uuid.NewSHA1(pijectorV1Space, uuid.NodeID())
)

func localScreenID(addr string) string {
	return uuid.NewSHA1(localKioskSpace, []byte(addr)).String()
}

// ScreenStatus contains information about a Screen's current display.
type ScreenStatus struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

// Screen represents a single Pijector display.
type Screen interface {
	// ID of the Screen, which is unique across Pijector instances.
	ID() string
	// Name of the Screen. Intended to be human-friendly. If name is not set when
	// the Screen is created, it will return ID().
	Name() string
	// Show a url on the Screen.
	Show(u string) error
	// Snap a screenshot of the Screen's current display.
	Snap() (io.ReadCloser, error)
	// Stat of the Screen.
	Stat() (ScreenStatus, error)
}

// localScreen controls a host-local Chromium instance via the Chrome Devtools
// Protocol.
type localScreen struct {
	addr, id, name string

	sync.RWMutex // protects following members
	browser      *rod.Browser
	current      *rod.Page
}

func (s *localScreen) ID() string {
	return s.id
}

func (s *localScreen) Name() string {
	if s.name == "" {
		return s.id
	}
	return s.name
}

func (s *localScreen) Show(u string) error {
	s.Lock()
	defer s.Unlock()
	var loadEvent proto.PageLoadEventFired
	if err := s.current.Navigate(u); err != nil {
		return err
	}
	s.current.WaitEvent(&loadEvent)()
	return nil
}

func (s *localScreen) Snap() (io.ReadCloser, error) {
	s.RLock()
	ssReq := &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	}
	data, err := s.current.Screenshot(false, ssReq)
	s.RUnlock()
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(data)), nil
}

func (s *localScreen) Stat() (ScreenStatus, error) {
	var stat ScreenStatus
	info, err := s.current.Info()
	if err != nil {
		return stat, err
	}
	stat.Title = info.Title
	stat.URL = info.URL
	return stat, nil
}

// AttachLocal attaches a local Chromium instance via CDP at the provided addr,
// and identifies it in Pijector with the provided human-friendly name.
func AttachLocal(name, addr string) (Screen, error) {
	u, err := launcher.ResolveURL(addr)
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
	return &localScreen{
		addr:    addr,
		id:      localScreenID(addr),
		name:    name,
		browser: browser,
		current: pages[0],
	}, nil
}

// remoteScreen uses the Pijector API to control a Screen attached locally to
// a different Pijector instance.
type remoteScreen struct {
	c                       *http.Client
	name, id, url, password string
}

func (s *remoteScreen) ID() string {
	return s.id
}

func (s *remoteScreen) Name() string {
	if s.name == "" {
		return s.id
	}
	return s.name
}

var errHTTPFailure = errors.New("http request failed")

func vetResponse(r *http.Response) error {
	if int(r.StatusCode/100) != 2 {
		return fmt.Errorf("%w: %v", errHTTPFailure, r.Status)
	}
	return nil
}

func (s *remoteScreen) Show(u string) error {
	reqURL := fmt.Sprintf("%v/show?target=%v", s.url, url.QueryEscape(u))
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.c.Do(req)
	if err != nil {
		return err
	}
	return vetResponse(resp)
}

func (s *remoteScreen) Snap() (io.ReadCloser, error) {
	reqURL := fmt.Sprintf("%v/snap", s.url)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.c.Do(req)
	if err != nil {
		return nil, err
	}
	if err := vetResponse(resp); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *remoteScreen) Stat() (stat ScreenStatus, err error) {
	reqURL := fmt.Sprintf("%v/stat", s.url)
	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return
	}
	var resp *http.Response
	resp, err = s.c.Do(req)
	if err != nil {
		return
	}
	if err = vetResponse(resp); err != nil {
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&stat)
	return
}

type remoteInitOpt struct {
	ClientTimeout time.Duration
	CookieJar     http.CookieJar
	Transport     http.RoundTripper
}

func defaultInitOptions() *remoteInitOpt {
	jar, _ := cookiejar.New(nil)
	return &remoteInitOpt{
		CookieJar:     jar,
		ClientTimeout: 2 * time.Second,
		Transport:     DefaultTransport(),
	}
}

// DefaultTransport for HTTP requests to the LB112X API. Useful for wrapping the
// transport from outside the library.
func DefaultTransport() http.RoundTripper {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Dial: (&net.Dialer{
			Timeout: 2 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 2 * time.Second,
	}
}

type RemoteOption func(*remoteInitOpt)

// WithRoundTripper for HTTP requests to the Pijector API.
func WithRoundTripper(rt http.RoundTripper) RemoteOption {
	return func(o *remoteInitOpt) {
		o.Transport = rt
	}
}

// WithClientTimeout while connecting to the Pijector API.
func WithClientTimeout(ttl time.Duration) RemoteOption {
	return func(o *remoteInitOpt) {
		o.ClientTimeout = ttl
	}
}

// WithCookieJar for use by the HTTP client communicating with the Pijector API.
func WithCookieJar(jar http.CookieJar) RemoteOption {
	return func(o *remoteInitOpt) {
		o.CookieJar = jar
	}
}

var errInvalidScreenURL = errors.New("invalid screen URL")

func extractV1RemoteScreenID(u string) (string, error) {
	parts := strings.Split(u, "/api/v1/screen/")
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: %v", errInvalidScreenURL, u)
	}
	return parts[1], nil
}

func AttachRemote(u, password string, opts ...RemoteOption) (Screen, error) {
	o := defaultInitOptions()
	for _, opt := range opts {
		opt(o)
	}
	id, err := extractV1RemoteScreenID(u)
	if err != nil {
		return nil, err
	}
	return &remoteScreen{
		c: &http.Client{
			Jar:       o.CookieJar,
			Transport: o.Transport,
			Timeout:   o.ClientTimeout,
		},
		id:       id,
		url:      u,
		password: password,
	}, nil
}

type Kiosk struct {
	id, name string
	screens  map[string]Screen
}

func (k *Kiosk) ID() string {
	return k.id
}

func (k *Kiosk) Name() string {
	return k.name
}

func (k *Kiosk) Screens() []string {
	disps := make([]string, len(k.screens))
	var i int
	for id := range k.screens {
		disps[i] = id
		i++
	}
	return disps
}

var errNoSuchScreen = errors.New("no such screen")

func (k *Kiosk) Screen(id string) (Screen, error) {
	d, has := k.screens[id]
	if !has {
		return nil, errNoSuchScreen
	}
	return d, nil
}

func New(name string, screens []Screen) *Kiosk {
	k := &Kiosk{
		name:    name,
		id:      localKioskSpace.String(),
		screens: make(map[string]Screen),
	}
	for _, s := range screens {
		k.screens[s.ID()] = s
	}
	return k
}
