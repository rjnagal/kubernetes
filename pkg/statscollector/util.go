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

// Utility methods to calculate percentiles from raw data.

package statscollector

import (
	"math"
	"sort"
	"time"

	"github.com/golang/glog"
	cadvisor "github.com/google/cadvisor/info"
)

const milliSecondsToNanoSeconds = 1000000
const secondsToMilliSeconds = 1000

type uint64Slice []uint64

func (a uint64Slice) Len() int           { return len(a) }
func (a uint64Slice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a uint64Slice) Less(i, j int) bool { return a[i] < a[j] }

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
