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

// Interface to periodically retrieve stats from all nodes in a cluster and provide
// aggregated summary. Current implementation report node-level stats.
// TODO(jnagal): Extend to report pod and container level stats summary.

package statscollector

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
)

// We poll each node for stats every minute.
const pollInterval = 1 * time.Minute

type Aggregator interface {
	// Start polling.
	Start() error

	// Stop polling.
	Stop() error

	// Get usage stats summary for the whole node.
	// Returns a map with hostname as key and NodeData as value.
	GetNodeStats() (map[string]NodeData, error)
}

type aggregator struct {
	dataLock         sync.RWMutex
	nodeApi          NodeApi
	clusterApi       Cluster
	nodes            map[string]NodeData
	housekeepingChan chan error
}

// Create a new aggregator.
func New(node NodeApi, cluster Cluster) (Aggregator, error) {
	if node == nil || cluster == nil {
		return nil, fmt.Errorf("nil node or cluster driver.")
	}

	newAggregator := &aggregator{
		nodes:      make(map[string]NodeData, 0),
		nodeApi:    node,
		clusterApi: cluster,
	}

	return newAggregator, nil
}

func (self *aggregator) Start() error {
	self.housekeepingChan = make(chan error)
	// process first update now.
	self.doUpdate()
	go self.periodicHousekeeping(self.housekeepingChan)
	return nil
}

func (self *aggregator) Stop() error {
	// Signal for housekeeping to exit.
	self.housekeepingChan <- nil
	err := <-self.housekeepingChan
	if err != nil {
		return err
	}
	return nil
}

func (self *aggregator) doUpdate() {
	// Check for new nodes.
	err := self.detectNodes()
	if err != nil {
		glog.Errorf("Failed to detect nodes: %s", err)
	} else {
		err := self.updateStats()
		if err != nil {
			glog.Errorf("Failed to update stats: %s", err)
		}
	}

}

func (self *aggregator) periodicHousekeeping(quit chan error) {
	ticker := time.Tick(pollInterval)
	for {
		select {
		case <-ticker:
			self.doUpdate()
		case <-quit:
			quit <- nil
			glog.Infof("Exiting housekeeping")
			return
		}
	}
}

func (self *aggregator) detectNodes() error {
	nodes, err := self.clusterApi.GetNodesList()
	if err != nil {
		return err
	}
	self.dataLock.Lock()
	defer self.dataLock.Unlock()
	for _, node := range nodes {
		_, ok := self.nodes[node.Name]
		if !ok {
			self.nodes[node.Name] = NodeData{
				Id: node,
			}
		}
	}
	return nil
}

func (self *aggregator) fixCpuUsage(capacity Capacity, resource Resource) Resource {
	// Due to the time difference between recording a timestamp and cpu usage,
	// cpu rate can go over machine capacity by a fraction. Ceil them off here.
	if resource.Cpu.Mean > capacity.Cpu {
		resource.Cpu.Mean = capacity.Cpu
	}
	if resource.Cpu.Max > capacity.Cpu {
		resource.Cpu.Max = capacity.Cpu
	}
	if resource.Cpu.Ninety > capacity.Cpu {
		resource.Cpu.Ninety = capacity.Cpu
	}
	return resource
}

func (self *aggregator) updateStats() error {
	// TODO(jnagal): Don't hold lock while making client calls.
	self.dataLock.Lock()
	defer self.dataLock.Unlock()
	for _, node := range self.nodes {
		// Update Capacity before usage.
		if node.Capacity.Cpu == 0 {
			glog.Infof("updating capacity for node %s", node.Id.Name)
			capacity, err := self.nodeApi.MachineSpec(node.Id)
			if err != nil {
				glog.Errorf("Failed to update capacity for node %s", node.Id.Name)
			} else {
				node.Capacity = capacity
			}
		}
		resource, err := self.nodeApi.UpdateStats(node.Id)
		if err != nil {
			glog.Errorf("Failed to update stats for node %s", node.Id.Name)
			// Mark old data as stale.
			node.Stats.MinuteUsage.Valid = false
			// Drop nodes that have not been updated in the past hour.
			if time.Since(node.Stats.LastUpdate).Hours() > 1 {
				glog.Errorf("Node %s presumed dead", node.Id.Name)
				delete(self.nodes, node.Id.Name)
			}
			continue
		}
		node.Stats.MinuteUsage = self.fixCpuUsage(node.Capacity, resource)
		// TODO: Calculate hour/day stats by storing minute stats.
		node.Stats.HourUsage.Valid = false
		node.Stats.DayUsage.Valid = false
		node.Stats.LastUpdate = time.Now()
		self.nodes[node.Id.Name] = node
	}
	return nil
}

func (self *aggregator) GetNodeStats() (map[string]NodeData, error) {
	self.dataLock.RLock()
	defer self.dataLock.RUnlock()
	return self.nodes, nil
}
