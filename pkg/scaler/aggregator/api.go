package aggregator

import "github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"

type Node struct {
	Capacity types.Resource
	Usage    types.Resource
	NumPods	 uint32
}

type Aggregator interface {
	// Returns a map with node name as key.
	GetClusterInfo() (map[string]Node, error)
}
