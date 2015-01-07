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

// Interface to provide machine capacity and raw stats (60 per-second samples).
// TODO(rjnagal): Extend to retrieve pod stats.

package statscollector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/golang/glog"
	cadvisor "github.com/google/cadvisor/info"
)

// Number of (per-second) stats to retrieve on each update.
const numStatsPerUpdate = 60

type NodeApi interface {
	UpdateStats(NodeId) (Resource, error)
	MachineSpec(NodeId) (Capacity, error)
}

type nodeApi struct {
	// Kubelet port used for retrieving node stats.
	kubeletPort int
}

func NewKubeNodeApi(kubeletPort int) (NodeApi, error) {
	return &nodeApi{
		kubeletPort: kubeletPort,
	}, nil
}

func GetValueFromResponse(response *http.Response, value interface{}) error {
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, value)
	if err != nil {
		return fmt.Errorf("Got '%s': %v", string(body), err)
	}
	return nil
}

func (self *nodeApi) getKubeletAddress(id NodeId) string {
	return "http://" + id.Address + ":" + strconv.Itoa(self.kubeletPort)
}

func (self *nodeApi) MachineSpec(id NodeId) (Capacity, error) {
	var machineInfo cadvisor.MachineInfo
	url := self.getKubeletAddress(id) + "/spec"
	resp, err := http.Get(url)
	if err != nil {
		return Capacity{}, err
	}
	err = GetValueFromResponse(resp, &machineInfo)
	if err != nil {
		glog.Errorf("Getting machine stats for minion %s with ip %s failed - %s\n", id.Name, id.Address, err)
		return Capacity{}, err
	}
	return Capacity{
		Memory: uint64(machineInfo.MemoryCapacity),
		// Convert cpu to MilliCpus for consistency with other data types.
		Cpu: uint64(machineInfo.NumCores) * 1000,
	}, nil
}

func (self *nodeApi) getMachineStats(id NodeId) ([]*cadvisor.ContainerStats, error) {
	var containerInfo cadvisor.ContainerInfo
	values := url.Values{}
	values.Add("num_stats", strconv.Itoa(numStatsPerUpdate))
	url := self.getKubeletAddress(id) + "/stats" + "?" + values.Encode()
	resp, err := http.Get(url)
	if err != nil {
		return []*cadvisor.ContainerStats{}, err
	}
	err = GetValueFromResponse(resp, &containerInfo)
	if err != nil {
		glog.Errorf("Updating Stats for minion %s with ip %s failed - %s\n", id.Name, id.Address, err)
		return []*cadvisor.ContainerStats{}, err
	}

	return containerInfo.Stats, nil
}

func (self *nodeApi) UpdateStats(id NodeId) (Resource, error) {
	stats, err := self.getMachineStats(id)
	if err != nil {
		return Resource{}, err
	}

	cpu, memory := GetPercentiles(stats)
	return Resource{
		Valid:  true,
		Cpu:    cpu,
		Memory: memory,
	}, nil
}

type fakeNodeApi struct {
}

func NewFakeNodeApi() (NodeApi, error) {
	return &fakeNodeApi{}, nil
}

func (self *fakeNodeApi) MachineSpec(id NodeId) (Capacity, error) {
	return Capacity{
		Cpu:    8000,
		Memory: 8 * 1024 * 1024 * 1024,
	}, nil
}

func (self *fakeNodeApi) UpdateStats(id NodeId) (Resource, error) {
	cpu := Percentiles{
		Mean:   15,
		Max:    161,
		Ninety: 123,
	}
	memory := Percentiles{
		Mean:   1073741824,
		Max:    9663676416,
		Ninety: 7516192768,
	}
	return Resource{
		Valid:  true,
		Cpu:    cpu,
		Memory: memory,
	}, nil
}
