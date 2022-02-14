package cni

import (
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/services"
	"github.com/containerd/go-cni"
	"github.com/pkg/errors"
)

// networkAttachCount is the minimum number of networks the PodSandbox
// attaches to
const networkAttachCount = 1

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.ServicePlugin,
		ID:   services.CNIService,
		Requires: []plugin.Type{
			plugin.EventPlugin,
			plugin.MetadataPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			//m, err := ic.Get(plugin.MetadataPlugin)
			//if err != nil {
			//	return nil, err
			//}
			//ep, err := ic.Get(plugin.EventPlugin)
			//if err != nil {
			//	return nil, err
			//}

			dir := "/etc/cni/net.d"
			max := 1
			networkPluginBinDir := "/opt/cni/bin"
			i, err := cni.New(cni.WithMinNetworkCount(networkAttachCount),
				cni.WithPluginConfDir(dir),
				cni.WithPluginMaxConfNum(max),
				cni.WithPluginDir([]string{networkPluginBinDir}))
			if err != nil {
				return nil, errors.Wrap(err, "failed to initialize cni")
			}
			err = i.Load(cni.WithDefaultConf)
			if err != nil {
				return nil, errors.Wrap(err, "failed to initialize cni")
			}

			return &cniService{
				NetconfPath: "/etc/cni/net.d",
				NetNSDir:    "/var/run/netns",
				netPlugin:   i,
			}, nil
		},
	})
}
