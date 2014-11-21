package actuator

import (
	"fmt"
	"math"

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
	// Returns the default node shape type.
	GetDefaultNodeShape() (string, error)
	// Creates a new nodes based on the input nodeShapeName and returns the hostname of the new node.
	CreateNode(nodeShapeName string) (string, error)
}

// Represents all the node shapes available.
type NodeShapes struct {
	nodeCapacityToShape map[types.Resource]string
}

func abs(val int64) uint64 {
	return uint64(math.Abs(float64(val)))
}

// Returns a node shape if 'capacity' maps to a legal node shape, error otherwise.
func (self *NodeShapes) GetNodeShapeWithCapacity(expectedCapacity types.Resource) (NodeShape, error) {
	// TODO(vishh): There is a mismatch in the capacity returned by the actuator and the aggregator. Simplify the code once the two services agree with each other.
	var nodeShape NodeShape
	for capacity, shape := range self.nodeCapacityToShape {
		if nodeShape.Name == "" {
			nodeShape = NodeShape{shape, capacity}
			continue
		}
		newCpuDelta := abs(int64(capacity.Cpu) - int64(expectedCapacity.Cpu))
		oldCpuDelta := abs(int64(nodeShape.Capacity.Cpu) - int64(expectedCapacity.Cpu))
		newMemoryDelta := abs(int64(capacity.Memory) - int64(expectedCapacity.Memory))
		oldMemoryDelta := abs(int64(nodeShape.Capacity.Memory) - int64(expectedCapacity.Memory))
		if newCpuDelta < oldCpuDelta || newMemoryDelta < oldMemoryDelta {
			nodeShape = NodeShape{shape, capacity}
		}
	}

	return nodeShape, nil
}

// Returns a node shape if 'shapeType' maps to a legal node shape, error otherwise.
func (self *NodeShapes) GetNodeShapeWithType(shapeType string) (NodeShape, error) {
	for capacity, shape := range self.nodeCapacityToShape {
		if shape == shapeType {
			return NodeShape{shape, capacity}, nil
		}
	}

	return NodeShape{}, fmt.Errorf("unrecognized node shape with type: %s", shapeType)
}

func (self *NodeShapes) add(capacity types.Resource, shape string) {
	self.nodeCapacityToShape[capacity] = shape
}

func newNodeShapes() NodeShapes {
	return NodeShapes{
		nodeCapacityToShape: make(map[types.Resource]string),
	}
}
