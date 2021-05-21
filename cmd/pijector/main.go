package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cfunkhouser/pijector"
	"github.com/cfunkhouser/pijector/api"
	"github.com/cfunkhouser/pijector/client"
	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
)

var (
	// Version of pijector. Overridden at build time.
	Version = "development"

	defaultKioskAddress    = "127.0.0.1:9222"
	defaultListenAddress   = "0.0.0.0:9292"
	defaultKioskDisplayURL = "http://localhost:9292/"
)

func serveAPI(c *cli.Context) error {
	kioskAddrs := c.StringSlice("kiosk")
	if len(kioskAddrs) < 1 {
		return cli.Exit("kiosk must be set", 1)
	}
	d := c.String("default-url")
	if d == "" {
		return cli.Exit("default-url must be set", 1)
	}
	var kiosks []*pijector.Kiosk
	for _, ka := range kioskAddrs {
		k, err := pijector.Dial(ka)
		if err != nil {
			return cli.Exit(err, 1)
		}
		kiosks = append(kiosks, k)
		go func() { _ = k.Show(d) }()
	}
	r := mux.NewRouter()
	api.ConfigureV1(r, kiosks)
	r.PathPrefix("/").HandlerFunc(client.StaticHandler)
	http.Handle("/", r)
	return http.ListenAndServe(c.String("listen"), nil)
}

func show(c *cli.Context) error {
	k := c.String("kiosk")
	if k == "" {
		return cli.Exit("kiosk must be set", 1)
	}
	d := c.String("default-url")
	if d == "" {
		return cli.Exit("default-url must be set", 1)
	}
	ksk, err := pijector.Dial(k)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if err := ksk.Show(d); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func snap(c *cli.Context) error {
	k := c.String("kiosk")
	if k == "" {
		return cli.Exit("kiosk must be set", 1)
	}
	o := c.String("output")
	if o == "" {
		return cli.Exit("output must be set", 1)
	}
	ksk, err := pijector.Dial(k)
	if err != nil {
		return cli.Exit(err, 1)
	}
	data, err := ksk.Screenshot()
	if err != nil {
		return cli.Exit(err, 1)
	}
	of, err := os.Create(o)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if _, err := of.Write(data); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func main() {
	app := &cli.App{
		Name:    "pijector",
		Usage:   "Turns a Chromium browser in debug mode into a Kiosk display.",
		Version: Version,
		Commands: []*cli.Command{
			{
				Name: "server",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "kiosk",
						Aliases: []string{"k"},
						Usage:   "ip:port on which Chromium Kiosk's debugger is listening",
						EnvVars: []string{"BILLBOARD_KIOSK_ADDRESS"},
						Value:   cli.NewStringSlice(defaultKioskAddress),
					},
					&cli.StringFlag{
						Name:    "listen",
						Aliases: []string{"L"},
						Usage:   "ip:port on which to serve API requests",
						Value:   defaultListenAddress,
					},
					&cli.StringFlag{
						Name:    "default-url",
						Aliases: []string{"d"},
						Usage:   "Default URL to open in the Chromium Kiosk",
						EnvVars: []string{"BILLBOARD_KIOSK_DEFAULT_URL"},
						Value:   defaultKioskDisplayURL,
					},
				},
				Usage:  "Run the pijector server",
				Action: serveAPI,
			},
			{
				Name: "show",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "kiosk",
						Aliases: []string{"k"},
						Usage:   "ip:port on which Chromium Kiosk's debugger is listening",
						EnvVars: []string{"BILLBOARD_KIOSK_ADDRESS"},
						Value:   defaultKioskAddress,
					},
					&cli.StringFlag{
						Name:    "default-url",
						Aliases: []string{"d"},
						Usage:   "Default URL to open in the Chromium Kiosk",
						EnvVars: []string{"BILLBOARD_KIOSK_DEFAULT_URL"},
						Value:   defaultKioskDisplayURL,
					},
				},
				Usage:  "Instruct the Kiosk to display a URL.",
				Action: show,
			},
			{
				Name: "snap",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "kiosk",
						Aliases: []string{"k"},
						Usage:   "ip:port on which Chromium Kiosk's debugger is listening",
						EnvVars: []string{"BILLBOARD_KIOSK_ADDRESS"},
						Value:   defaultKioskAddress,
					},
					&cli.StringFlag{
						Name:     "output",
						Required: true,
						Aliases:  []string{"o"},
						Usage:    "Filename to which the PNG output is written.",
					},
				},
				Usage:  "Take a screenshot of the Kiosk's current display.",
				Action: snap,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
