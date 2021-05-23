package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cfunkhouser/pijector"
	"github.com/cfunkhouser/pijector/admin"
	"github.com/cfunkhouser/pijector/api"
	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
)

var (
	defaultPijectorListenAddr = "0.0.0.0:9292"
	defaultPijectorScreenAddr = "127.0.0.1:9222"
	defaultPijectorScreenURL  = "http://localhost:9292/"

	commonFlags = []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "screen",
			Aliases: []string{"s"},
			Usage:   "ip:port on which a Chromium debugger is listening. May be repeated.",
			EnvVars: []string{"PIJECTOR_SCREEN_ADDRESS"},
			Value:   cli.NewStringSlice(defaultPijectorScreenAddr),
		},
		&cli.StringFlag{
			Name:    "default-url",
			Aliases: []string{"d"},
			Usage:   "Default URL to open on Pijector Screens",
			EnvVars: []string{"PIJECTOR_SCREEN_DEFAULT_URL"},
			Value:   defaultPijectorScreenURL,
		},
	}
)

func getCommonFlags(c *cli.Context) (ps []string, d string, err error) {
	ps = c.StringSlice("screen")
	if len(ps) < 1 {
		err = cli.Exit("at least one screen must be specified", 1)
		return
	}
	d = c.String("default-url")
	if d == "" {
		err = cli.Exit("default-url must be set", 1)
	}
	return
}

func serve(c *cli.Context) error {
	done := make(chan error)
	ps, d, err := getCommonFlags(c)
	if err != nil {
		return err
	}
	var screens []pijector.Screen
	for _, saddr := range ps {
		s, err := pijector.AttachLocal("", saddr)
		if err != nil {
			return cli.Exit(err, 1)
		}
		screens = append(screens, s)
	}
	r := mux.NewRouter()
	api.HandleV1(r, screens)
	r.PathPrefix("/").HandlerFunc(admin.Handler)
	http.Handle("/", r)

	go func(errs chan<- error, addr string) {
		errs <- http.ListenAndServe(c.String("listen"), nil)
	}(done, c.String("listen"))

	for _, s := range screens {
		_ = s.Show(d)
	}

	err = <-done
	return err
}

func show(c *cli.Context) error {
	ps, d, err := getCommonFlags(c)
	if err != nil {
		return err
	}
	for _, saddr := range ps {
		s, err := pijector.AttachLocal("", saddr)
		if err != nil {
			return cli.Exit(err, 1)
		}
		if err := s.Show(d); err != nil {
			return cli.Exit(err, 1)
		}
	}
	return nil
}

func snap(c *cli.Context) error {
	ps, _, err := getCommonFlags(c)
	if err != nil {
		return err
	}
	if len(ps) != 1 {
		return cli.Exit("snap really only makes sense with one screen", 1)
	}
	s, err := pijector.AttachLocal("", ps[0])
	if err != nil {
		return cli.Exit(err, 1)
	}
	o := c.String("output")
	if o == "" {
		return cli.Exit("output must be set", 1)
	}
	snap, err := s.Snap()
	if err != nil {
		return cli.Exit(err, 1)
	}
	of, err := os.Create(o)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if _, err := io.Copy(of, snap); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func main() {
	app := &cli.App{
		Name:    "pijector",
		Usage:   "Turns a Chromium browser in debug mode into a Kiosk display.",
		Version: pijector.Version,
		Commands: []*cli.Command{
			{
				Name: "server",
				Flags: append(commonFlags, &cli.StringFlag{
					Name:    "listen",
					Aliases: []string{"L"},
					Usage:   "ip:port on which to serve API requests",
					Value:   defaultPijectorListenAddr,
				}),
				Usage:  "Run the pijector server",
				Action: serve,
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
