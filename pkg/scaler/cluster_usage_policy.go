package scaler

import (
	"flag"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/aggregator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
	"github.com/golang/glog"
)

var argThreshold = flag.Uint("cluster_threshold", 90, "Percentage of cluster resource usage beyond which the cluster size will be increased.")

// TODO(vishh): Consider replacing minute/hour/day with intent - aggresive/moderate/conservative.
var argScalingPolicy = flag.String("cluster_policy", "hour", "Cluster nodes will be scaled based on usage for the last minute, hour or day. Choose between 'minute' (aggresive), 'hour' (moderate) and 'day' (conservative).")

const (
	minute int = iota
	hour
	day
)

type clusterUsagePolicy struct {
	threshold     uint
	scalingPolicy int
}

// Returns the percentage of value over limit.
func PercentageOf(value, limit uint64) uint {
	return uint(((limit - value) * 100) / limit)
}

// Returns the stats required by the scaling policy if available.
func (self *clusterUsagePolicy) getUsageBasedOnPolicy(derivedStats aggregator.DerivedStats) *types.Resource {
	switch self.scalingPolicy {
	case minute:
		if derivedStats.MinuteUsage.Valid {
			return &types.Resource{derivedStats.MinuteUsage.Cpu.Ninety, derivedStats.MinuteUsage.Memory.Ninety}
		}
	case hour:
		if derivedStats.HourUsage.Valid {
			return &types.Resource{derivedStats.HourUsage.Cpu.Ninety, derivedStats.HourUsage.Memory.Ninety}
		}
	case day:
		if derivedStats.DayUsage.Valid {
			return &types.Resource{derivedStats.DayUsage.Cpu.Ninety, derivedStats.DayUsage.Memory.Ninety}
		}
	}

	return nil
}

func (self *clusterUsagePolicy) PerformScaling(cluster *Cluster) error {
	nodesAboveThreshold := 0
	stableNodes := 0
	for _, node := range cluster.Current {
		// TODO(vishh): Handle nodes that have been offline for a while, and hence no recent stats.
		usage := self.getUsageBasedOnPolicy(node.Usage)
		if usage == nil {
			// Skipping node since latest stats are not available which happens when the node is unresponsive.
			continue
		} 
		stableNodes++
		if PercentageOf(usage.Cpu, node.Capacity.Cpu) >= self.threshold ||
			PercentageOf(usage.Memory, node.Capacity.Memory) >= self.threshold {
			glog.V(2).Infof("Host %s is using more than %d of its capacity", node.Hostname, self.threshold)
			nodesAboveThreshold++
		}
		cluster.Slack.Cpu += (node.Capacity.Cpu - usage.Cpu)
		cluster.Slack.Memory += (node.Capacity.Memory - usage.Memory)
	}
	if nodesAboveThreshold > 0 && PercentageOf(uint64(nodesAboveThreshold), uint64(stableNodes)) > self.threshold {
		if len(cluster.New) == 0 {
			glog.Infof("%d nodes in the cluster are above their threshold resource usage. Increasing cluster size by one node.", nodesAboveThreshold)
			// Increase the cluster size by one node.
			cluster.New = append(cluster.New, cluster.DefaultShape.Name)
		}
	}

	return nil
}

func newClusterUsagePolicy() (Policy, error) {
	if *argThreshold <= 0 {
		return nil, fmt.Errorf("Cluster scaling threshold invalid: %d", *argThreshold)
	}
	scalingPolicy := hour
	switch *argScalingPolicy {
	case "minute":
		scalingPolicy = minute
	case "day":
		scalingPolicy = day
	default:
		glog.Warningf("Cluster scaling policy not set via flag --cluster_policy. Defaulting to moderate scaling.")
	}

	return &clusterUsagePolicy{
		threshold:     *argThreshold,
		scalingPolicy: scalingPolicy,
	}, nil
}
