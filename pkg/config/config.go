package config

import (
	"encoding/json"
	"io/ioutil"

	"github.com/golang/glog"
)

type DummyConfig struct {
	Plugins []PluginConfig `json:"plugins"`
}

type PluginConfig struct {
	Name         string   `json:"name"`
	ResourceName string   `json:"resourceName"`
	Devices      []Device `json:"devices"`
}

type Device struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

func (d *DummyConfig) ParseFromFile(file string) error {
	glog.Info("Discovering dummy devices")
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		glog.Errorf("failed to read file: %s, error: %s", file, err.Error())
		return err
	}
	err = json.Unmarshal(raw, d)
	if err != nil {
		glog.Errorf("failed to Unmarshal: %s,", err.Error())
		return err
	}
	return nil
}
