package scaler

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/actuator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/aggregator"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
)

type Scaler interface {
	AutoScale() error
}

type Node struct {
	aggregator.Node
	// The node shape name that uniquely identifies this node type.
	shapeName string
}

type Cluster struct {
	Shapes       actuator.NodeShapes
	DefaultShape actuator.NodeShape
	// Map of hostname to node
	Current map[string]Node
	// List of node shapes.
	New   []string
	Slack types.Resource
}

type Policy interface {
	// Analyze the current state of cluster and scale the cluster if necessary.
	// Arguments:
	//   *Cluster: Contains the current state of the cluster and scaling that needs to be performed.
	// Updates 'New' and 'Slack' fields of the input on success, error otherwise.
	PerformScaling(*Cluster) error
}
