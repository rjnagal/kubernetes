package main

import (
	"flag"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler"
	"github.com/golang/glog"
)

var argHousekeepingTick = flag.Duration("housekeeping", 1*time.Minute, "Housekeeping duration.")

var argThreshold = flag.Uint("cluster_threshold", 90, "Percentage of cluster resource usage beyond which the cluster size will be increased.")

// TODO(vishh): Consider replacing minute/hour/day with intent - aggresive/moderate/conservative.
var argClusterScalingPolicy = flag.String("cluster_scaling_policy", "hour", "Cluster nodes will be scaled based on usage for the last minute, hour or day. Choose between 'minute' (aggresive), 'hour' (moderate) and 'day' (conservative).")

var argActuatorHostPort = flag.String("actuator_hostport", "localhost:8080", "Actuator Host:Port.")

var argAggregatorHostPort = flag.String("aggregator_hostport", "localhost:8085", "Aggregator Host:Port.")

func main() {
	flag.Parse()
	autoScaler, err := scaler.New(*argHousekeepingTick, *argActuatorHostPort, *argAggregatorHostPort, *argClusterScalingPolicy, *argThreshold)
	if err != nil {
		glog.Fatal(err)
	}
	if err = autoScaler.AutoScale(); err != nil {
		glog.Fatal(err)
	}
}
