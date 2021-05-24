package main

import (
	"io"
	"strings"

	"github.com/cfunkhouser/pijector"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type screenConfig struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Address string `json:"address" yaml:"address"`
	// TODO(cfunkhouser): Add password for remote screens.
}

func naivelyIsRemote(addr string) bool {
	return strings.Contains(addr, "/api/v1/screen/")
}

func (c *screenConfig) attach() (pijector.Screen, error) {
	if naivelyIsRemote(c.Address) {
		return pijector.AttachRemote(c.Name, c.Address)
	}
	return pijector.AttachLocal(c.Name, c.Address)
}

type serverConfig struct {
	Listen     string         `json:"listen" yaml:"listen"`
	DefaultURL string         `json:"default_url" yaml:"default_url"`
	Screens    []screenConfig `json:"screens" yaml:"screens"`
}

var (
	defaultPijectorListenAddr = "0.0.0.0:9292"
	defaultPijectorScreenAddr = "127.0.0.1:9222"
	defaultPijectorScreenURL  = "http://localhost:9292/"

	defaultServerConfig = serverConfig{
		Listen:     defaultPijectorListenAddr,
		DefaultURL: defaultPijectorScreenURL,
	}
)

func Load(r io.Reader) (*serverConfig, error) {
	config := defaultServerConfig
	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return nil, err
	}
	logrus.Debugf("Loaded config: %+v", config)
	return &config, nil
}
