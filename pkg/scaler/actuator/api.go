package actuator

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
)

type NodeShape struct {
	// Unique type name assigned for this shape by the Cloud provider.
	Name string
	// Resouces available as part of this shape.
	Capacity types.Resource
}

type Actuator interface {
	// Returns all the available node shapes for this cluster.
	GetNodeShapes() (NodeShapes, error)
	// Returns the default node shape.
	GetDefaultNodeShape() (NodeShape, error)
	// Creates a new nodes based on the input nodeShapeName and returns the hostname of the new node.
	CreateNode(nodeShapeName string) (string, error)
}

// Represents all the node shapes available.
type NodeShapes struct {
	nodeShapes map[types.Resource]NodeShape
}

// Returns a node shape if 'capacity' maps to a legal node shape, error otherwise.
func (self *NodeShapes) GetNodeShape(capacity types.Resource) (NodeShape, error) {
	if nodeShape, ok := self.nodeShapes[capacity]; ok {
		return nodeShape, nil
	}
	return NodeShape{}, fmt.Errorf("unrecognized node shape with capacity: %+v", capacity)
}

func (self *NodeShapes) addNodeShape(nodeShape NodeShape) {
	self.nodeShapes[nodeShape.Capacity] = nodeShape
}

func newNodeShapes() NodeShapes {
	return NodeShapes{
		nodeShapes: make(map[types.Resource]NodeShape),
	}
}
