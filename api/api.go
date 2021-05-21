// Package api is the HTTP API hander for a pijector server.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cfunkhouser/pijector"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const pijectorV1NamespaceUUI = "ed77ce29-9f8f-4d4c-87c3-91078f49e9f9"

var spaceV1 = uuid.Must(uuid.Parse(pijectorV1NamespaceUUI))

func kioskID(k *pijector.Kiosk) string {
	return uuid.NewSHA1(spaceV1, []byte(k.Address())).String()
}

type v1KioskHandler struct {
	k *pijector.Kiosk
}

func (v *v1KioskHandler) getShow(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("target")
	if u == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "target parameter is required")
		return
	}
	if !(strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
		u = fmt.Sprintf("http://%v", u)
	}
	saneURL, err := url.ParseRequestURI(u)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Provided target %q is not a real URL", u)
		return
	}
	if err := v.k.Show(saneURL.String()); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "The Kiosk couldn't display %q: %v", u, err)
		return
	}
	v.getStat(w, r)
}

func (v *v1KioskHandler) getSnap(w http.ResponseWriter, r *http.Request) {
	data, err := v.k.Screenshot()
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "The Kiosk couldn't provide a screenshot: %v", err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

type status struct {
	pijector.Details
	ScreenshotURL string `json:"screenshot,omitempty"`
}

func cacheproofSnapURL(k *pijector.Kiosk) string {
	n := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("/api/v1/kiosk/%v/snap?%v", kioskID(k), n)
}

func (v *v1KioskHandler) getStat(w http.ResponseWriter, r *http.Request) {
	deets, err := v.k.Details()
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "The Kiosk couldn't provide a screenshot: %v", err)
		return
	}
	es := status{
		Details:       *deets,
		ScreenshotURL: cacheproofSnapURL(v.k),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(&es); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func v1HandleKiosk(router *mux.Router, k *pijector.Kiosk) {
	r := router.PathPrefix(fmt.Sprintf("/kiosk/%v", kioskID(k))).Subrouter().StrictSlash(true)
	api := &v1KioskHandler{
		k: k,
	}
	r.Methods(http.MethodGet).Path("/").HandlerFunc(api.getStat)
	r.Methods(http.MethodGet).Path("/show").HandlerFunc(api.getShow)
	r.Methods(http.MethodGet).Path("/snap").HandlerFunc(api.getSnap)
	r.Methods(http.MethodGet).Path("/stat").HandlerFunc(api.getStat)
}

type kioskDetails struct {
	URL  string `json:"url"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type v1 struct {
	kioskDetails []*kioskDetails
}

func (v *v1) getKiosks(w http.ResponseWriter, r *http.Request) {
	response := map[string][]*kioskDetails{
		"kiosks": v.kioskDetails,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

const v1APIPrefix = "/api/v1"

func ConfigureV1(router *mux.Router, kiosks []*pijector.Kiosk) {
	r := router.PathPrefix(v1APIPrefix).Subrouter().StrictSlash(true)
	api := &v1{
		kioskDetails: make([]*kioskDetails, len(kiosks)),
	}
	for i, k := range kiosks {
		v1HandleKiosk(r, k)
		kid := kioskID(k)
		api.kioskDetails[i] = &kioskDetails{
			URL:  fmt.Sprintf("/api/v1/kiosk/%v", kid),
			ID:   kid,
			Name: k.Address(),
		}
	}
	r.Methods(http.MethodGet).Path("/kiosk").HandlerFunc(api.getKiosks)
}

func New(kiosks []*pijector.Kiosk) http.Handler {
	r := mux.NewRouter()
	ConfigureV1(r, kiosks)
	return r
}
