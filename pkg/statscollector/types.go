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
	"time"
)

type Percentiles struct {
	// Average over the collected sample.
	Mean uint64 `json:"mean"`
	// Max seen over the collected sample.
	Max uint64 `json:"max"`
	// 90th percentile over the collected sample.
	Ninety uint64 `json:"ninety"`
}

type Capacity struct {
	// Number of available cpus. In milliCpus for consistency.
	Cpu int `json:"cpu"`
	// Amount of memory in bytes.
	Memory uint64 `json:"memory"`
}

type Resource struct {
	// Mean, Max, and 90p cpu rate value in milliCpus/seconds. Converted to milliCpus to avoid floats.
	Cpu Percentiles `json:"cpu"`
	// Mean, Max, and 90p memory size in bytes.
	Memory Percentiles `json:"memory"`
}

type NodeId struct {
	// node name
	Name string `json:"name"`

	// Host ip for the node api.
	Address string `json:"address"`
}

type NodeStats struct {
	// Time since the last stats update. If a node is flaky, we'll keep it in the known node list for an hour.
	// Update time indicate how stale the stats are from that node.
	LastUpdate time.Time `json:"last_update"`
	// Percentiles in last (observed) minute.
	MinuteUsage Resource `json:"minute_usage"`
	// Percentile in last hour, barring node outages.
	HourUsage Resource `json:"hour_usage"`
	// Percentile in last day, barring node outages.
	DayUsage Resource `json:"day_usage"`
}

type NodeData struct {
	Capacity Capacity  `json:"capacity"`
	Id       NodeId    `json:"id"`
	Stats    NodeStats `json:"stats"`
}
