package cni

import (
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/services"
)

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
			return &cniService{
				NetconfPath: "/etc/cni/net.d",
			}, nil
		},
	})
}
