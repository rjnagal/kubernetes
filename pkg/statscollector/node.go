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
	"flag"
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

var numStatsPerUpdate = flag.Int("num_stats_per_update", 60, "Number of (per-second) stats to retrieve on each update.")
var kubeletPort = flag.Int("kubelet_port", 10250, "Kubelet port")

type NodeApi interface {
	UpdateStats(NodeId) (Resource, error)
	MachineSpec(NodeId) (Capacity, error)
}

type nodeApi struct {
}

func NewKubeNodeApi() (NodeApi, error) {
	return &nodeApi{}, nil
}

func PostRequestAndGetValue(client *http.Client, req *http.Request, value interface{}) error {
	response, err := client.Do(req)
	if err != nil {
		return err
	}
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

func getKubeletAddress(id NodeId) string {
	return "http://" + id.Address + ":" + strconv.Itoa(*kubeletPort)
}

func (self *nodeApi) MachineSpec(id NodeId) (Capacity, error) {
	var machineInfo cadvisor.MachineInfo
	url := getKubeletAddress(id) + "/spec"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Capacity{}, err
	}
	err = PostRequestAndGetValue(&http.Client{}, req, &machineInfo)
	if err != nil {
		glog.Errorf("Getting machine stats for minion %s with ip %s failed - %s\n", id.Name, id.Address, err)
		return Capacity{}, err
	}
	return Capacity{
		Memory: uint64(machineInfo.MemoryCapacity),
		// Convert cpu to MilliCpus for consistency with other data types.
		Cpu: machineInfo.NumCores * 1000,
	}, nil
}

func getMachineStats(id NodeId) ([]*cadvisor.ContainerStats, error) {
	var containerInfo cadvisor.ContainerInfo
	values := url.Values{}
	values.Add("num_stats", strconv.Itoa(*numStatsPerUpdate))
	url := getKubeletAddress(id) + "/stats" + "?" + values.Encode()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []*cadvisor.ContainerStats{}, err
	}
	err = PostRequestAndGetValue(&http.Client{}, req, &containerInfo)
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
	last_cpu := uint64(0)
	var last_time time.Time
	memory_samples := make(uint64Slice, len(stats))
	cpu_samples := make(uint64Slice, len(stats)-1)
	num_samples := 0
	memoryPercentiles := Percentiles{}
	cpuPercentiles := Percentiles{}
	for _, stat := range stats {
		num_samples++
		cpuNs := stat.Cpu.Usage.Total
		time := stat.Timestamp
		// Ignore actual usage and only focus on working set.
		memory := stat.Memory.WorkingSet
		if memory > memoryPercentiles.Max {
			memoryPercentiles.Max = memory
		}
		memoryPercentiles.Mean = GetMean(memoryPercentiles.Mean, memory, uint64(num_samples))
		memory_samples = append(memory_samples, memory)
		if last_cpu == 0 {
			last_cpu = cpuNs
			last_time = time
			continue
		}
		elapsed := time.Nanosecond() - last_time.Nanosecond()
		if elapsed < 10*milliSecondsToNanoSeconds {
			continue
		}
		cpu_rate := (cpuNs - last_cpu) * secondsToMilliSeconds / uint64(elapsed)
		if cpu_rate < 0 {
			continue
		}
		cpu_samples = append(cpu_samples, cpu_rate)
		if cpu_rate > cpuPercentiles.Max {
			cpuPercentiles.Max = cpu_rate
		}
		cpuPercentiles.Mean = GetMean(cpuPercentiles.Mean, cpu_rate, uint64(num_samples-1))
	}
	cpuPercentiles.Ninety = Get90Percentile(cpu_samples)
	memoryPercentiles.Ninety = Get90Percentile(memory_samples)
	return cpuPercentiles, memoryPercentiles
}

func (self *nodeApi) UpdateStats(id NodeId) (Resource, error) {
	stats, err := getMachineStats(id)
	if err != nil {
		return Resource{}, err
	}

	cpu, memory := GetPercentiles(stats)
	return Resource{
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
		Cpu:    cpu,
		Memory: memory,
	}, nil
}
