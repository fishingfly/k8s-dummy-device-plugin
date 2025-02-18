package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/fishingfly/k8s-dummy-device-plugin/pkg/config"
	"github.com/fishingfly/k8s-dummy-device-plugin/pkg/dummy"
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	conf := &config.DummyConfig{}
	if err := conf.ParseFromFile("./dummyResources.json"); err != nil {
		glog.Fatal(err)
	}

	ddms := []*dummy.DummyDeviceManager{}

	for _, p := range conf.Plugins {
		ddm := &dummy.DummyDeviceManager{
			Devices:      make(map[string]*pluginapi.Device),
			Socket:       pluginapi.DevicePluginPath + fmt.Sprintf("%s.sock", p.Name),
			Health:       make(chan *pluginapi.Device),
			ResoueceName: p.ResourceName,
		}
		for _, dev := range p.Devices {
			newdev := pluginapi.Device{ID: dev.Name, Health: pluginapi.Healthy}
			ddm.Devices[dev.Name] = &newdev
		}

		ddms = append(ddms, ddm)
	}

	// Respond to syscalls for termination
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for _, ddm := range ddms {
		// Start grpc server
		err := ddm.Start()
		if err != nil {
			glog.Fatalf("Could not start device plugin: %v", err)
		}
		glog.Infof("Starting to serve on %s", ddm.Socket)

		// Registers with Kubelet.
		err = ddm.Register()
		if err != nil {
			glog.Fatal(err)
		}
		glog.Info("device-plugin registered\n")
	}

	select {
	case s := <-sigs:
		glog.Infof("Received signal \"%v\", shutting down.", s)
		for _, ddm := range ddms {
			ddm.Stop()
		}

		return
	}
}
