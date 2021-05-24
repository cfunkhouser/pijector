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
	// Version of Pijector. Overridden at build time.
	Version = "development"

	pijectorGlobalSpace = uuid.Must(uuid.Parse("ed77ce29-9f8f-4d4c-87c3-91078f49e9f9"))
	pijectorLocalSpace  = uuid.NewSHA1(pijectorGlobalSpace, uuid.NodeID())
)

func localScreenID(addr string) string {
	return uuid.NewSHA1(pijectorLocalSpace, []byte(addr)).String()
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

	sync.Mutex // protects following members
	browser    *rod.Browser
	current    *rod.Page
}

// attachIfNecessary connects to the chromium debugger lazily, when needed. This
// allows the Pijector to be initialized before the Screen is actually available.
// This function assumes the lock is held before calling.
func (s *localScreen) attachIfNecessary() error {
	u, err := launcher.ResolveURL(s.addr)
	if err != nil {
		return err
	}
	browser := rod.New().ControlURL(u).DefaultDevice(devices.Clear)
	if err := browser.Connect(); err != nil {
		return err
	}
	pages, err := browser.Pages()
	if err != nil {
		return err
	}
	if _, err := pages[0].Activate(); err != nil {
		return err
	}
	s.browser = browser
	s.current = pages[0]
	return nil
}

func (s *localScreen) ID() string {
	return s.id
}

func (s *localScreen) Name() string {
	if s.name == "" {
		return s.addr
	}
	return s.name
}

func (s *localScreen) Show(u string) error {
	s.Lock()
	defer s.Unlock()
	if err := s.attachIfNecessary(); err != nil {
		return err
	}
	var loadEvent proto.PageLoadEventFired
	if err := s.current.Navigate(u); err != nil {
		return err
	}
	s.current.WaitEvent(&loadEvent)()
	return nil
}

func (s *localScreen) Snap() (io.ReadCloser, error) {
	s.Lock()
	if err := s.attachIfNecessary(); err != nil {
		return nil, err
	}
	ssReq := &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	}
	data, err := s.current.Screenshot(false, ssReq)
	s.Unlock()
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(data)), nil
}

func (s *localScreen) Stat() (ScreenStatus, error) {
	var stat ScreenStatus

	s.Lock()
	defer s.Unlock()
	if err := s.attachIfNecessary(); err != nil {
		return stat, err
	}
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
	return &localScreen{
		addr: addr,
		id:   localScreenID(addr),
		name: name,
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

type apiStat struct {
	Display ScreenStatus `json:"display"`
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
	var full apiStat
	err = json.NewDecoder(resp.Body).Decode(&full)
	stat = full.Display
	return
}

type remoteInitOpt struct {
	ClientTimeout time.Duration
	Transport     http.RoundTripper
	Password      string
}

func defaultInitOptions() *remoteInitOpt {
	return &remoteInitOpt{
		ClientTimeout: 2 * time.Second,
		Transport:     DefaultTransport(),
	}
}

// DefaultTransport for HTTP requests to the Pijector API. Useful for wrapping the
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

// WithPassword to authenticate to remote Pijector API.
func WithPassword(password string) RemoteOption {
	return func(o *remoteInitOpt) {
		o.Password = password
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

func AttachRemote(name, u string, opts ...RemoteOption) (Screen, error) {
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
			Transport: o.Transport,
			Timeout:   o.ClientTimeout,
		},
		name:     name,
		id:       id,
		url:      u,
		password: o.Password,
	}, nil
}
