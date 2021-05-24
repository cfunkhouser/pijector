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
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
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
	cp := c.String("config")
	if cp == "" {
		return cli.Exit("server needs a config", 1)
	}
	cf, err := os.Open(cp)
	if err != nil {
		return cli.Exit(err, 1)
	}
	cfg, err := Load(cf)
	if err != nil {
		return cli.Exit(err, 1)
	}

	var screens []pijector.Screen
	for _, scfg := range cfg.Screens {
		s, err := scfg.attach()
		if err != nil {
			logrus.WithError(err).WithField("address", scfg.Address).Warn("attach failed")
			// return cli.Exit(err, 1)
		} else {
			logrus.WithField("address", scfg.Address).Info("attached to screen")
		}
		screens = append(screens, s)
	}

	r := mux.NewRouter()
	api.HandleV1(r, screens)
	r.PathPrefix("/").HandlerFunc(admin.Handler)
	http.Handle("/", r)

	done := make(chan error)
	go func(errs chan<- error, addr string) {
		logrus.Infof("server listening on %v", cfg.Listen)
		errs <- http.ListenAndServe(cfg.Listen, nil)
	}(done, cfg.Listen)

	for _, s := range screens {
		if err := s.Show(cfg.DefaultURL); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"target": cfg.DefaultURL,
				"screen": s.ID(),
			}).Warning("show failed")
		}
	}

	err = <-done
	logrus.WithError(err).Infof("server done listening")
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
	logrus.SetLevel(logrus.DebugLevel)
	app := &cli.App{
		Name:    "pijector",
		Usage:   "Turns a Chromium browser in debug mode into a Kiosk display.",
		Version: pijector.Version,
		Commands: []*cli.Command{
			{
				Name: "server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "config",
						Required: true,
						Aliases:  []string{"c"},
						Usage:    "Path to server configuration file.",
					},
				},
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
