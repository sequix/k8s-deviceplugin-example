package dp

import (
	"fmt"
	"net"
	"os"
	"path"
	"time"
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	dpapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	nDevices = 5
	resourceName   = "sequix.cn/example"
	devicePath     = "/dev/mydev"
	pluginEndpoint = "sequix-dp.sock"
	socketPath     = dpapi.DevicePluginPath + pluginEndpoint
)

// Plugin implements device plugin interface
type Plugin struct {
	devices map[string]*dpapi.Device
	stop    chan interface{}
	server  *grpc.Server
}

// NewPlugin creates Plugin
func NewPlugin() *Plugin {
	m := &Plugin{
		devices: make(map[string]*dpapi.Device),
		stop:    make(chan interface{}),
	}
	for i := 0; i < nDevices; i++ {
		id := fmt.Sprintf("dev%03d", i)
		m.devices[id] = &dpapi.Device{ID: id, Health: dpapi.Healthy}
	}
	return m
}

// Run starts gRPC server and register device plugin to Kubelet
func (p *Plugin) Run() error {
	log.Printf("DevicePlugin: Run")
	
	err := p.Start()
	if err != nil {
		log.Printf("Failed to start device plugin: %v", err)
		return err
	}

	log.Printf("Device plugin socket path: %s", socketPath)

	err = p.Register()
	if err != nil {
		log.Printf("Failed to register device plugin: %v", err)
		p.Stop()
		return err
	}

	log.Printf("Device plugin is running")

	return nil
}

// Start starts gRPC server
func (p *Plugin) Start() error {
	log.Printf("DevicePlugin: Start")

	if err := p.cleanup(); err != nil {
		return err
	}
	sock, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	p.server = grpc.NewServer([]grpc.ServerOption{}...)
	dpapi.RegisterDevicePluginServer(p.server, p)
	go p.server.Serve(sock)

	return nil
}

// Stop stops gRPC server
func (p *Plugin) Stop() error {
	log.Printf("DevicePlugin: Stop")

	if p.server == nil {
		return nil
	}
	p.server.Stop()
	p.server = nil
	close(p.stop)
	return p.cleanup()
}

// Register registers device plugin with kubelet
func (p *Plugin) Register() error {
	log.Printf("DevicePlugin: Register")

	conn, err := grpc.Dial(dpapi.KubeletSocket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := dpapi.NewRegistrationClient(conn)

	req := &dpapi.RegisterRequest{
		Version:      dpapi.Version,
		Endpoint:     path.Base(socketPath),
		ResourceName: resourceName,
	}
	_, err = client.Register(context.Background(), req)
	if err != nil {
		return err
	}
	return nil
}

// ListAndWatch implements DevicePlugin API call
func (p *Plugin) ListAndWatch(emtpy *dpapi.Empty, stream dpapi.DevicePlugin_ListAndWatchServer) error {
	log.Printf("DevicePlugin: ListAndWatch")

	resp := new(dpapi.ListAndWatchResponse)
	for _, dev := range p.devices {
		resp.Devices = append(resp.Devices, dev)
	}
	if err := stream.Send(resp); err != nil {
		log.Printf("ListAndWatch failed to send responce to kubelet. Error: %v", err)
		return err
	}
	for {
		select {
		case <-p.stop:
			log.Printf("DevicePlugin: ListAndWatch exit.")
			return nil
		}
	}
}

// Allocate implements DevicePlugin API call
func (p *Plugin) Allocate(ctx context.Context, req *dpapi.AllocateRequest) (*dpapi.AllocateResponse, error) {
	log.Printf("DevicePlugin: Allocate")
	resp := new(dpapi.AllocateResponse)
	for _, creq := range req.ContainerRequests {
		cresp := new(dpapi.ContainerAllocateResponse)
		log.Printf("Request devices: %v", creq.DevicesIDs)
		cresp.Devices = append(cresp.Devices, &dpapi.DeviceSpec{
			HostPath:      "/dev/null",
			ContainerPath: devicePath,
			Permissions:   "rw",
		})
		resp.ContainerResponses = append(resp.ContainerResponses, cresp)
	}
	return resp, nil
}

// GetDevicePluginOptions implements DevicePlugin API call
func (p *Plugin) GetDevicePluginOptions(context.Context, *dpapi.Empty) (*dpapi.DevicePluginOptions, error) {
	log.Printf("DevicePlugin: GetDevicePluginOptions")
	return &dpapi.DevicePluginOptions{}, nil
}

// PreStartContainer implements DevicePlugin API call
func (p *Plugin) PreStartContainer(context.Context, *dpapi.PreStartContainerRequest) (*dpapi.PreStartContainerResponse, error) {
	log.Printf("DevicePlugin: PreStartContainer")
	return &dpapi.PreStartContainerResponse{}, nil
}

// GetPreferredAllocation implements DevicePlugin API call
func (p *Plugin) GetPreferredAllocation(context.Context, *dpapi.PreferredAllocationRequest) (*dpapi.PreferredAllocationResponse, error) {
	log.Printf("DevicePlugin: GetPreferredAllocation")
	return &dpapi.PreferredAllocationResponse{}, nil
}

func (p *Plugin) cleanup() error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}