// Package api is the HTTP API hander for a pijector server.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cfunkhouser/pijector"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type v1ScreenHandler struct {
	s pijector.Screen
}

func (v *v1ScreenHandler) getShow(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("target")
	if u == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "target parameter is required")
		logrus.WithField("client", r.RemoteAddr).Info("bad request, no target")
		return
	}
	if !(strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
		u = fmt.Sprintf("http://%v", u)
	}
	saneURL, err := url.ParseRequestURI(u)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "target %q is not a real URL", u)
		logrus.WithError(err).WithField("client", r.RemoteAddr).Info("bad request, bad target")
		return
	}
	if err := v.s.Show(saneURL.String()); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "screen couldn't show %q: %v", u, err)
		logrus.WithError(err).WithField("client", r.RemoteAddr).Warn("show failed")
		return
	}
	v.getStat(w, r)
}

func (v *v1ScreenHandler) getSnap(w http.ResponseWriter, r *http.Request) {
	snap, err := v.s.Snap()
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "screen couldn't provide a snapshot: %v", err)
		logrus.WithError(err).WithField("client", r.RemoteAddr).Warn("snap failed")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = io.Copy(w, snap)
}

type screenDetail struct {
	URL     string                `json:"url"`
	ID      string                `json:"id"`
	Name    string                `json:"name,omitempty"`
	SnapURL string                `json:"snap,omitempty"`
	Display pijector.ScreenStatus `json:"display"`
}

func cacheproofSnapURL(s pijector.Screen) string {
	n := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("/api/v1/screen/%v/snap?%v", s.ID(), n)
}

func screenDetails(s pijector.Screen) (*screenDetail, error) {
	stat, err := s.Stat()
	if err != nil {
		return nil, err
	}
	sid := s.ID()
	return &screenDetail{
		URL:     fmt.Sprintf("/api/v1/screen/%v", sid),
		ID:      sid,
		Name:    s.Name(),
		SnapURL: cacheproofSnapURL(s),
		Display: stat,
	}, nil
}

func screenToJSON(s pijector.Screen) ([]byte, error) {
	m, err := screenDetails(s)
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func (v *v1ScreenHandler) getStat(w http.ResponseWriter, r *http.Request) {
	data, err := screenToJSON(v.s)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "screen couldn't provide status: %v", err)
		logrus.WithError(err).WithField("client", r.RemoteAddr).Warn("stat failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		// Not much else we can do at this point.
		logrus.WithError(err).WithField("client", r.RemoteAddr).Error("returning stat payload failed")
	}
}

func v1HandleScreen(router *mux.Router, s pijector.Screen) {
	r := router.PathPrefix(fmt.Sprintf("/screen/%v", s.ID())).Subrouter().StrictSlash(true)
	api := &v1ScreenHandler{
		s: s,
	}
	r.Methods(http.MethodGet).Path("/").HandlerFunc(api.getStat)
	r.Methods(http.MethodGet).Path("/show").HandlerFunc(api.getShow)
	r.Methods(http.MethodGet).Path("/snap").HandlerFunc(api.getSnap)
	r.Methods(http.MethodGet).Path("/stat").HandlerFunc(api.getStat)
}

type v1 struct {
	screens []pijector.Screen
}

type screensPayload struct {
	Screens []*screenDetail `json:"screens"`
}

func (v *v1) getScreens(w http.ResponseWriter, r *http.Request) {
	var sp screensPayload
	for _, s := range v.screens {
		deets, err := screenDetails(s)
		if err != nil {
			logrus.WithError(err).WithField("screen", s.ID()).Warn("skipping unreachable screen")
			continue
		}
		sp.Screens = append(sp.Screens, deets)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(&sp); err != nil {
		// Not much else we can do at this point.
		logrus.WithError(err).WithField("client", r.RemoteAddr).Error("returning screens payload failed")
	}
}

const V1APIPrefix = "/api/v1"

// HandleV1 API at V1APIPrefix under the router.
func HandleV1(router *mux.Router, screens []pijector.Screen) {
	r := router.PathPrefix(V1APIPrefix).Subrouter().StrictSlash(true)
	for _, s := range screens {
		v1HandleScreen(r, s)
	}
	r.Methods(http.MethodGet).Path("/screen").HandlerFunc((&v1{
		screens: screens,
	}).getScreens)
}

// New V1 Pijector API handler.
func New(screens []pijector.Screen) http.Handler {
	r := mux.NewRouter()
	HandleV1(r, screens)
	return r
}
