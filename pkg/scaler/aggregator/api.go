package aggregator

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/statscollector"
)

type DerivedStats statscollector.DerivedStats

type Container struct {
	Name  string
	Limit types.Resource
	Usage DerivedStats
}

type Pod struct {
	Name       string
	ID         string
	Containers []*Container
	Status     string
}

type Node struct {
	Hostname string
	Capacity types.Resource
	Usage    DerivedStats
	Pods     []Pod
}

type Aggregator interface {
	// Returns a map of hostname to Node, for all the hosts in the cluster.
	GetClusterInfo() (map[string]Node, error)
}
