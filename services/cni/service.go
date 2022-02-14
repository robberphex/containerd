package cni

import (
	"context"
	"github.com/containerd/containerd/pkg/netns"
	gocni "github.com/containerd/go-cni"
	"github.com/containernetworking/cni/libcni"
	cniv1 "github.com/containernetworking/cni/pkg/types/v1"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"os"
	"sort"
	"strings"
)

// grpcServices are all the grpc services provided by cri containerd.
type grpcServices interface {
	cniv1.CNIServer
}

// CNIService is the interface implement CRI remote service server.
type CNIService interface {
	Register(*grpc.Server) error
	grpcServices
}

// cniService implements CNIService.
type cniService struct {
	cniv1.UnimplementedCNIServer

	NetconfPath string
	NetNSDir    string
	netPlugin   gocni.CNI
}

func (c cniService) Register(server *grpc.Server) error {
	cniv1.RegisterCNIServer(server, c)
	return nil
}

func (c cniService) ListNetworkConfig(ctx context.Context, request *cniv1.ListNetworkConfigRequest) (*cniv1.ListNetworkConfigReply, error) {
	listReply := &cniv1.ListNetworkConfigReply{}
	if _, err := os.Stat(c.NetconfPath); err != nil {
		if os.IsNotExist(err) {
			return listReply, nil
		}
		return nil, err
	}
	fileNames, err := libcni.ConfFiles(c.NetconfPath, []string{".conf", ".conflist", ".json"})
	if err != nil {
		return nil, err
	}
	sort.Strings(fileNames)
	for _, fileName := range fileNames {
		var lcl *libcni.NetworkConfigList
		if strings.HasSuffix(fileName, ".conflist") {
			lcl, err = libcni.ConfListFromFile(fileName)
			if err != nil {
				return nil, err
			}
			listReply.Network = append(listReply.Network, &cniv1.ListNetworkConfigReply_NetworkConfigNameAndType{
				Name:       lcl.Name,
				Type:       cniv1.ListNetworkConfigReply_NetworkConfigNameAndType_CONFIG,
				ConfigPath: fileName,
			})
		} else {
			lc, err := libcni.ConfFromFile(fileName)
			if err != nil {
				return nil, err
			}
			listReply.Network = append(listReply.Network, &cniv1.ListNetworkConfigReply_NetworkConfigNameAndType{
				Name:       lc.Network.Name,
				Type:       cniv1.ListNetworkConfigReply_NetworkConfigNameAndType_CONFIG,
				ConfigPath: fileName,
			})
			//lcl, err = libcni.ConfListFromConf(lc)
			//if err != nil {
			//	return nil, err
			//}
		}
	}

	return listReply, nil
}

func (c cniService) AddNetworkList(ctx context.Context, request *cniv1.AddNetworkListRequest) (*cniv1.AddNetworkListReply, error) {
	err := c.netPlugin.Status()
	if err != nil {
		return nil, err
	}
	var netw *gocni.ConfNetwork
	for _, network := range c.netPlugin.GetConfig().Networks {
		if network.Config.Name == request.GetName() {
			netw = network
		}
	}
	if netw == nil {
		return nil, errors.New("not found ")
	}

	netPath := request.GetRuntimeConf().GetNetNS()
	if netPath == "" {
		var netnsMountDir = "/var/run/netns"
		netNS, err := netns.NewNetNS(netnsMountDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create network namespace for sandbox %q", request.GetRuntimeConf().GetContainerID())
		}
		netPath = netNS.GetPath()
	}
	res, err := c.netPlugin.Setup(ctx, "id", netPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Setup Network error")
	}
	_ = res

	reply := &cniv1.AddNetworkListReply{}
	reply.NetNS = netPath
	return reply, nil
}

var _ CNIService = &cniService{}
