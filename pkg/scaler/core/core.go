package core

import (
	"flag"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/actuator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/aggregator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
	"github.com/golang/glog"
)

type realAutoScaler struct {
	// A map of policy name to Policy
	policies         map[string]Policy
	nodeShapes       actuator.NodeShapes
	defaultNodeShape actuator.NodeShape
	actuator         actuator.Actuator
	aggregator       aggregator.Aggregator
	// Map of hostname to Node information.
	existingNodes map[string]Node
	// Map of hostname to shape type.
	newNodes map[string]string
}

var argHousekeepingTick = flag.Duration("housekeeping", 10*time.Minute, "Housekeeping duration.")

func (self *realAutoScaler) AutoScale() error {
	for {
		err := self.doHousekeeping()
		if err != nil {
			glog.Error(err)
		}
		// Sleep for housekeeping duration.
		time.Sleep(*argHousekeepingTick)
	}
	return nil
}

func (self *realAutoScaler) doHousekeeping() error {
	hostnameToNodesMap, err := self.aggregator.GetClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to get cluster node information from aggregator - %q", err)
	}

	cluster, err := self.applyPolicies(hostnameToNodesMap)
	if err != nil {
		return err
	}

	err = self.handleClusterResizing(cluster)
	if err != nil {
		return err
	}

	// TODO(vishh): Surface slack resources/nodes.
	return nil
}

func (self *realAutoScaler) applyPolicies(hostnameToNodesMap map[string]aggregator.Node) (*Cluster, error) {
	clusterNodes := make(map[string]Node, 0)
	for _, node := range hostnameToNodesMap {
		nodeShape, err := self.nodeShapes.GetNodeShape(node.Capacity)
		if err != nil {
			glog.Fatal(err)
		}
		clusterNodes[node.Hostname] = Node{node, nodeShape.Name}
	}
	cluster := &Cluster{
		Shapes:       self.nodeShapes,
		DefaultShape: self.defaultNodeShape,
		Current:      clusterNodes,
		New:          make([]string, 0),
		Slack:        types.Resource{0, 0},
	}

	for title, policy := range self.policies {
		glog.V(1).Infof("Applying policy %s", title)
		glog.V(2).Infof("Cluster: %+v", cluster)
		err := policy.PerformScaling(cluster)
		if err != nil {
			// TODO(vishh): Move on to applying other policies instead.
			return nil, err
		}
		glog.V(2).Infof("Cluster after applying policy %s: %+v", title, cluster)
	}

	return cluster, nil
}

func New() (Scaler, error) {
	myActuator := actuator.New()
	nodeShapes, err := myActuator.GetNodeShapes()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing node shapes %q", err)
	}
	defaultNodeShape, err := myActuator.GetDefaultNodeShape()
	if err != nil {
		return nil, fmt.Errorf("failed to get default node shape %q", err)
	}
	// List policies in the order of increasing priority
	policies := map[string]Policy{
		"ClusterUsage": &clusterUsagePolicy{},
	}

	return &realAutoScaler{
		policies:         policies,
		aggregator:       aggregator.New(),
		actuator:         myActuator,
		nodeShapes:       nodeShapes,
		defaultNodeShape: defaultNodeShape,
	}, nil
}
