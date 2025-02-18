package dummy

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

// DummyDeviceManager manages our dummy devices
type DummyDeviceManager struct {
	Devices      map[string]*pluginapi.Device
	Socket       string
	Server       *grpc.Server
	Health       chan *pluginapi.Device
	ResoueceName string
}

// Init function for our dummy devices
func (ddm *DummyDeviceManager) Init() error {
	glog.Info("Initializing dummy device plugin...")
	return nil
}

// Register with kubelet
func (ddm *DummyDeviceManager) Register() error {
	conn, err := grpc.Dial(pluginapi.KubeletSocket, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	defer conn.Close()
	if err != nil {
		return fmt.Errorf("device-plugin: cannot connect to kubelet service: %v", err)
	}
	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version: pluginapi.Version,
		// Name of the unix socket the device plugin is listening on
		// PATH = path.Join(DevicePluginPath, endpoint)
		Endpoint: filepath.Base(ddm.Socket),
		// Schedulable resource name.
		ResourceName: ddm.ResoueceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return fmt.Errorf("device-plugin: cannot register to kubelet service: %v", err)
	}
	return nil
}

// Start starts the gRPC server of the device plugin
func (ddm *DummyDeviceManager) Start() error {
	err := ddm.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", ddm.Socket)
	if err != nil {
		return err
	}

	ddm.Server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(ddm.Server, ddm)

	go ddm.Server.Serve(sock)

	// Wait for server to start by launching a blocking connection
	conn, err := grpc.Dial(ddm.Socket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		return err
	}

	conn.Close()

	go ddm.healthcheck()

	return nil
}

// Stop stops the gRPC server
func (ddm *DummyDeviceManager) Stop() error {
	if ddm.Server == nil {
		return nil
	}

	ddm.Server.Stop()
	ddm.Server = nil

	return ddm.cleanup()
}

// healthcheck monitors and updates device status
// TODO: Currently does nothing, need to implement
func (ddm *DummyDeviceManager) healthcheck() error {
	for {
		glog.Info(ddm.Devices)
		time.Sleep(60 * time.Second)
	}
}

func (ddm *DummyDeviceManager) cleanup() error {
	if err := os.Remove(ddm.Socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// ListAndWatch lists devices and update that list according to the health status
func (ddm *DummyDeviceManager) ListAndWatch(emtpy *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	glog.Info("device-plugin: ListAndWatch start\n")
	resp := new(pluginapi.ListAndWatchResponse)
	for _, dev := range ddm.Devices {
		glog.Info("dev ", dev)
		resp.Devices = append(resp.Devices, dev)
	}
	glog.Info("resp.Devices ", resp.Devices)
	if err := stream.Send(resp); err != nil {
		glog.Errorf("Failed to send response to kubelet: %v", err)
	}

	for {
		select {
		case d := <-ddm.Health:
			d.Health = pluginapi.Unhealthy
			resp := new(pluginapi.ListAndWatchResponse)
			for _, dev := range ddm.Devices {
				glog.Info("dev ", dev)
				resp.Devices = append(resp.Devices, dev)
			}
			glog.Info("resp.Devices ", resp.Devices)
			if err := stream.Send(resp); err != nil {
				glog.Errorf("Failed to send response to kubelet: %v", err)
			}
		}
	}
}

// Allocate devices
func (ddm *DummyDeviceManager) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	glog.Info("Allocate")
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		for _, id := range req.DevicesIDs {
			if _, ok := ddm.Devices[id]; !ok {
				glog.Errorf("Can't allocate interface %s", id)
				return nil, fmt.Errorf("invalid allocation request: unknown device: %s", id)
			}
		}
		glog.Info("Allocated interfaces ", req.DevicesIDs)
		response := pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{"DUMMY_DEVICES": strings.Join(req.DevicesIDs, ",")},
		}
		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}
	return &responses, nil
}

// GetDevicePluginOptions returns options to be communicated with Device Manager
func (ddm *DummyDeviceManager) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase,
// before each container start. Device plugin can run device specific operations
// such as reseting the device before making devices available to the container
func (ddm *DummyDeviceManager) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}
