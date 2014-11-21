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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/golang/glog"
	cadvisor "github.com/google/cadvisor/info"
)

const milliSecondsToNanoSeconds = 1000000
const secondsToMilliSeconds = 1000

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

// TODO(jnagal): Move all stats related code to its own util package.
// Get 90th percentile of the provided samples. Round to integer.
func Get90Percentile(samples uint64Slice) uint64 {
	count := len(samples)
	if count == 0 {
		return 0
	}
	sort.Sort(samples)
	n := float64(0.9 * (float64(count) + 1))
	idx, frac := math.Modf(n)
	index := int(idx)
	percentile := float64(samples[index-1])
	if index > 1 || index < count {
		percentile += frac * float64(samples[index]-samples[index-1])
	}
	return uint64(percentile)
}

// Add new sample to existing average. Round to integer.
func GetMean(mean uint64, value uint64, count uint64) uint64 {
	if count < 1 {
		return 0
	}
	return (mean*(count-1) + value) / count
}

type uint64Slice []uint64

func (a uint64Slice) Len() int           { return len(a) }
func (a uint64Slice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a uint64Slice) Less(i, j int) bool { return a[i] < a[j] }

func GetPercentiles(stats []*cadvisor.ContainerStats) (Percentiles, Percentiles) {
	lastCpu := uint64(0)
	var lastTime time.Time
	memorySamples := make(uint64Slice, len(stats))
	cpuSamples := make(uint64Slice, len(stats)-1)
	numSamples := 0
	memoryPercentiles := Percentiles{}
	cpuPercentiles := Percentiles{}
	for _, stat := range stats {
		numSamples++
		cpuNs := stat.Cpu.Usage.Total
		time := stat.Timestamp
		// Ignore actual usage and only focus on working set.
		memory := stat.Memory.WorkingSet
		if memory > memoryPercentiles.Max {
			memoryPercentiles.Max = memory
		}
		glog.V(2).Infof("Read sample: cpu %d, memory %d", cpuNs, memory)
		memoryPercentiles.Mean = GetMean(memoryPercentiles.Mean, memory, uint64(numSamples))
		memorySamples = append(memorySamples, memory)
		if lastCpu == 0 {
			lastCpu = cpuNs
			lastTime = time
			continue
		}
		elapsed := time.UnixNano() - lastTime.UnixNano()
		if elapsed < 10*milliSecondsToNanoSeconds {
			glog.Infof("Elasped time too small: %d ns: time now %s last %s", elapsed, time.String(), lastTime.String())
			continue
		}
		cpuRate := (cpuNs - lastCpu) * secondsToMilliSeconds / uint64(elapsed)
		if cpuRate < 0 {
			glog.Infof("cpu rate too small: %f ns", cpuRate)
			continue
		}
		glog.V(2).Infof("Adding cpu rate sample : %d", cpuRate)
		lastCpu = cpuNs
		lastTime = time
		cpuSamples = append(cpuSamples, cpuRate)
		if cpuRate > cpuPercentiles.Max {
			cpuPercentiles.Max = cpuRate
		}
		cpuPercentiles.Mean = GetMean(cpuPercentiles.Mean, cpuRate, uint64(numSamples-1))
	}
	cpuPercentiles.Ninety = Get90Percentile(cpuSamples)
	memoryPercentiles.Ninety = Get90Percentile(memorySamples)
	return cpuPercentiles, memoryPercentiles
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
