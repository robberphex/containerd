package cni

import (
	"context"
	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types/v2"
	"google.golang.org/grpc"
	"os"
	"sort"
	"strings"
)

// grpcServices are all the grpc services provided by cri containerd.
type grpcServices interface {
	v2.CNIServer
}

// CNIService is the interface implement CRI remote service server.
type CNIService interface {
	Register(*grpc.Server) error
	grpcServices
}

// cniService implements CNIService.
type cniService struct {
	v2.UnimplementedCNIServer

	NetconfPath string
}

func (c cniService) Register(server *grpc.Server) error {
	v2.RegisterCNIServer(server, c)
	return nil
}

func (c cniService) ListNetworkConfig(ctx context.Context, request *v2.ListNetworkConfigRequest) (*v2.ListNetworkConfigReply, error) {
	listReply := &v2.ListNetworkConfigReply{}
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
			listReply.Network = append(listReply.Network, &v2.ListNetworkConfigReply_NetworkConfigNameAndType{
				Name:       lcl.Name,
				Type:       v2.ListNetworkConfigReply_NetworkConfigNameAndType_CONFIG,
				ConfigPath: fileName,
			})
		} else {
			lc, err := libcni.ConfFromFile(fileName)
			if err != nil {
				return nil, err
			}
			listReply.Network = append(listReply.Network, &v2.ListNetworkConfigReply_NetworkConfigNameAndType{
				Name:       lc.Network.Name,
				Type:       v2.ListNetworkConfigReply_NetworkConfigNameAndType_CONFIG,
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

var _ CNIService = &cniService{}
