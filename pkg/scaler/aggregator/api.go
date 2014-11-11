package aggregator

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
)

type Container struct {
	Name  string
	Limit types.Resource
	Usage types.Resource
}

type Pod struct {
	Name       string       `json:"name,omitempty"`
	ID         string       `json:"id,omitempty"`
	Containers []*Container `json:"containers"`
	Status     string       `json:"status,omitempty"`
}

type Node struct {
	Hostname string
	Capacity types.Resource
	Usage    types.Resource
	Pods     []Pod
}

type Aggregator interface {
	// Returns a map of hostname to Node, for all the hosts in the cluster.
	GetClusterInfo() (map[string]Node, error)
}
