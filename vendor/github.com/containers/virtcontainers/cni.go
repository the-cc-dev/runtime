//
// Copyright (c) 2016 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package virtcontainers

import (
	cniPlugin "github.com/containers/virtcontainers/pkg/cni"
	"github.com/sirupsen/logrus"
)

// cni is a network implementation for the CNI plugin.
type cni struct{}

// Logger returns a logrus logger appropriate for logging cni messages
func (n *cni) Logger() *logrus.Entry {
	return virtLog.WithField("subsystem", "cni")
}

func (n *cni) addVirtInterfaces(networkNS *NetworkNamespace) error {
	netPlugin, err := cniPlugin.NewNetworkPlugin()
	if err != nil {
		return err
	}

	for idx, endpoint := range networkNS.Endpoints {
		virtualEndpoint, ok := endpoint.(*VirtualEndpoint)
		if !ok {
			continue
		}

		result, err := netPlugin.AddNetwork(virtualEndpoint.NetPair.ID, networkNS.NetNsPath, virtualEndpoint.NetPair.VirtIface.Name)
		if err != nil {
			return err
		}

		virtualEndpoint.SetProperties(*result)
		networkNS.Endpoints[idx] = virtualEndpoint

		n.Logger().Infof("AddNetwork results %v", *result)
	}

	return nil
}

func (n *cni) deleteVirtInterfaces(networkNS NetworkNamespace) error {
	netPlugin, err := cniPlugin.NewNetworkPlugin()
	if err != nil {
		return err
	}

	for _, endpoint := range networkNS.Endpoints {
		virtualEndpoint, ok := endpoint.(*VirtualEndpoint)
		if !ok {
			continue
		}

		err := netPlugin.RemoveNetwork(virtualEndpoint.NetPair.ID, networkNS.NetNsPath, virtualEndpoint.NetPair.VirtIface.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

// init initializes the network, setting a new network namespace for the CNI network.
func (n *cni) init(config NetworkConfig) (string, bool, error) {
	return initNetworkCommon(config)
}

// run runs a callback in the specified network namespace.
func (n *cni) run(networkNSPath string, cb func() error) error {
	return runNetworkCommon(networkNSPath, cb)
}

// add adds all needed interfaces inside the network namespace for the CNI network.
func (n *cni) add(pod Pod, config NetworkConfig, netNsPath string, netNsCreated bool) (NetworkNamespace, error) {
	endpoints, err := createNetworkEndpoints(config.NumInterfaces)
	if err != nil {
		return NetworkNamespace{}, err
	}

	networkNS := NetworkNamespace{
		NetNsPath:    netNsPath,
		NetNsCreated: netNsCreated,
		Endpoints:    endpoints,
	}

	if err := n.addVirtInterfaces(&networkNS); err != nil {
		return NetworkNamespace{}, err
	}

	if err := addNetworkCommon(pod, &networkNS); err != nil {
		return NetworkNamespace{}, err
	}

	return networkNS, nil
}

// remove unbridges and deletes TAP interfaces. It also removes virtual network
// interfaces and deletes the network namespace for the CNI network.
func (n *cni) remove(pod Pod, networkNS NetworkNamespace) error {
	if err := removeNetworkCommon(networkNS); err != nil {
		return err
	}

	if err := n.deleteVirtInterfaces(networkNS); err != nil {
		return err
	}

	return deleteNetNS(networkNS.NetNsPath, true)
}
