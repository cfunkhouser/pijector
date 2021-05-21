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

	commonFlags = []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "kiosk",
			Aliases: []string{"k"},
			Usage:   "ip:port on which Chromium Kiosk's debugger is listening",
			EnvVars: []string{"PIJECTOR_KIOSK_ADDRESS"},
			Value:   cli.NewStringSlice(defaultKioskAddress),
		},
		&cli.StringFlag{
			Name:    "default-url",
			Aliases: []string{"d"},
			Usage:   "Default URL to open in the Chromium Kiosk",
			EnvVars: []string{"PIJECTOR_KIOSK_DEFAULT_URL"},
			Value:   defaultKioskDisplayURL,
		},
	}
)

func getCommonFlags(c *cli.Context) (ks []string, d string, err error) {
	ks = c.StringSlice("kiosk")
	if len(ks) < 1 {
		err = cli.Exit("kiosk must be set", 1)
		return
	}
	d = c.String("default-url")
	if d == "" {
		err = cli.Exit("default-url must be set", 1)
	}
	return
}

func serveAPI(c *cli.Context) error {
	ks, d, err := getCommonFlags(c)
	if err != nil {
		return err
	}
	var kiosks []*pijector.Kiosk
	for _, ka := range ks {
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
	ks, d, err := getCommonFlags(c)
	if err != nil {
		return err
	}
	for _, ka := range ks {
		k, err := pijector.Dial(ka)
		if err != nil {
			return cli.Exit(err, 1)
		}
		if err := k.Show(d); err != nil {
			return cli.Exit(err, 1)
		}
	}
	return nil
}

func snap(c *cli.Context) error {
	ks, _, err := getCommonFlags(c)
	if err != nil {
		return err
	}
	if len(ks) != 1 {
		return cli.Exit("snap really only makes sense with one kiosk", 1)
	}
	k, err := pijector.Dial(ks[0])
	if err != nil {
		return cli.Exit(err, 1)
	}
	o := c.String("output")
	if o == "" {
		return cli.Exit("output must be set", 1)
	}
	data, err := k.Screenshot()
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
				Flags: append(commonFlags, &cli.StringFlag{
					Name:    "listen",
					Aliases: []string{"L"},
					Usage:   "ip:port on which to serve API requests",
					Value:   defaultListenAddress,
				}),
				Usage:  "Run the pijector server",
				Action: serveAPI,
			},
			{
				Name:   "show",
				Flags:  commonFlags,
				Usage:  "Instruct the Kiosk to display a URL.",
				Action: show,
			},
			{
				Name: "snap",
				Flags: append(commonFlags, &cli.StringFlag{
					Name:     "output",
					Required: true,
					Aliases:  []string{"o"},
					Usage:    "Filename to which the PNG output is written.",
				}),
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
