// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statscollector

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	kube_client "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/golang/glog"
)

type Cluster interface {
	GetNodesList() ([]NodeId, error)
}

type kubeCluster struct {
	client    *kube_client.Client
	lastQuery time.Time
	nodesList []NodeId
	dataLock  sync.RWMutex
}

func NewCluster(kubeMasterAddress string) (Cluster, error) {
	if len(kubeMasterAddress) == 0 {
		return nil, fmt.Errorf("Kubernetes master readonly API not specified.")
	}
	kubeClient := kube_client.NewOrDie(&kube_client.Config{
		Host:     "http://" + kubeMasterAddress,
		Version:  "v1beta1",
		Insecure: true,
	})

	return &kubeCluster{
		client: kubeClient,
	}, nil
}

// TODO(jnagal): Refactor this code in heapster and share.
func (self *kubeCluster) GetNodesList() ([]NodeId, error) {
	self.dataLock.Lock()
	defer self.dataLock.Unlock()
	// Avoid refreshing node list too often.
	if time.Since(self.lastQuery).Seconds() < 10 {
		return self.nodesList, nil
	}
	nodesList := make([]NodeId, 0)
	minions, err := self.client.Minions().List()
	if err != nil {
		return nil, err
	}
	for _, minion := range minions.Items {
		addrs, err := net.LookupIP(minion.Name)
		if err == nil {
			node := NodeId{
				Name:    minion.Name,
				Address: addrs[0].String(),
			}
			nodesList = append(nodesList, node)
		} else {
			glog.Errorf("Skipping host %s as IP lookup failed  - %s", minion.Name, err)
		}
	}
	self.lastQuery = time.Now()
	self.nodesList = nodesList
	return nodesList, nil
}

type fakeCluster struct {
	Nodes []NodeId
}

func NewFakeCluster(clusterSize int) (Cluster, error) {
	nodes := make([]NodeId, 0)
	for i := 0; i < clusterSize; i++ {
		name := "minion-" + strconv.Itoa(i)
		host := "1.0.0." + strconv.Itoa(i)
		node := NodeId{
			Name:    name,
			Address: host,
		}
		nodes = append(nodes, node)
	}
	return &fakeCluster{
		Nodes: nodes,
	}, nil
}

func (self *fakeCluster) GetNodesList() ([]NodeId, error) {
	return self.Nodes, nil
}
