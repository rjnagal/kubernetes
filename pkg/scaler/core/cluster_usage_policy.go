package core

import (
	"flag"

	"github.com/golang/glog"
)

type clusterUsagePolicy struct {
	threshold uint32
}

var argThreshold = flag.Int("cluster_threshold", 90, "Percentage of cluster resource usage beyond which the cluster size will be increased.")

// Returns the percentage of value over limit.
func PercentageOf(value, limit uint64) uint32 {
	return uint32(((limit - value) * 100) / limit)
}

func (self *clusterUsagePolicy) PerformScaling(cluster *Cluster) error {
	nodesAboveThreshold := 0
	for _, node := range cluster.Current {
		if PercentageOf(node.Usage.Cpu, node.Capacity.Cpu) >= self.threshold ||
			PercentageOf(node.Usage.Memory, node.Capacity.Memory) >= self.threshold {
			glog.V(2).Infof("Host %s is using more than %d of its capacity", node.Hostname, self.threshold)
			nodesAboveThreshold++
		}
		cluster.Slack.Cpu += (node.Capacity.Cpu - node.Usage.Cpu)
		cluster.Slack.Memory += (node.Capacity.Memory - node.Usage.Memory)
	}
	if PercentageOf(uint64(nodesAboveThreshold), uint64(len(cluster.Current))) > self.threshold {
		if len(cluster.New) == 0 {
			// A previous policy might have increased the cluster size. We do not want to increase it further here.
			glog.Infof("%d nodes in the cluster are above their threshold resource usage. Increasing cluster size by one node.", nodesAboveThreshold)
			// Increase the cluster size by one node.
			cluster.New = append(cluster.New, cluster.DefaultShape.Name)
		}
	}

	return nil
}

func newClusterUsagePolicy() Policy {
	return &clusterUsagePolicy{}
}
