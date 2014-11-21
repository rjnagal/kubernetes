package scaler

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/aggregator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
	"github.com/golang/glog"
)

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
	return 100 - uint(((limit-value)*100)/limit)
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
	default:
		glog.Fatal("Invalid cluster usage policy")
	}
	glog.V(1).Infof("no usage found based on policy")
	return nil
}

func (self *clusterUsagePolicy) PerformScaling(cluster *Cluster) error {
	nodesAboveThreshold := 0
	stableNodes := 0
	for _, node := range cluster.Current {
		glog.V(1).Infof("Checking usage of node: %s", node.Hostname)
		// TODO(vishh): Handle nodes that have been offline for a while, and hence no recent stats.
		usage := self.getUsageBasedOnPolicy(node.Usage)
		if usage == nil {
			// Skipping node since latest stats are not available which happens when the node is unresponsive.
			continue
		}
		glog.V(2).Infof("Node %s usage percentage cpu: %d, memory: %d", node.Hostname, PercentageOf(usage.Cpu, node.Capacity.Cpu),
			PercentageOf(usage.Memory, node.Capacity.Memory))

		stableNodes++
		glog.V(3).Infof("cpu usage percentage:")
		if PercentageOf(usage.Cpu, node.Capacity.Cpu) >= self.threshold ||
			PercentageOf(usage.Memory, node.Capacity.Memory) >= self.threshold {
			glog.V(2).Infof("Host %s is using more than %d of its capacity", node.Hostname, self.threshold)
			nodesAboveThreshold++
		} else {
			glog.V(1).Infof("Node %s is below threshold", node.Hostname)
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

func newClusterUsagePolicy(threshold uint, requestedScalingPolicy string) (Policy, error) {
	if threshold <= 0 {
		return nil, fmt.Errorf("Cluster scaling threshold invalid: %d", threshold)
	}
	scalingPolicy := hour
	switch requestedScalingPolicy {
	case "minute":
		scalingPolicy = minute
	case "day":
		scalingPolicy = day
	case "hour":
		break
	default:
		glog.Warningf("Cluster scaling policy not set via flag --cluster_scaling_policy. Defaulting to moderate scaling.")
	}

	glog.Infof("Cluster scaling threshold is set at %d", threshold)
	glog.Infof("Cluster scaling policy is %s", requestedScalingPolicy)
	return &clusterUsagePolicy{
		threshold:     threshold,
		scalingPolicy: scalingPolicy,
	}, nil
}
