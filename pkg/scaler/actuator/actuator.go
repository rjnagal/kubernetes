package actuator

import (
	"fmt"
)

type realActuator struct {
}

func (self *realActuator) GetNodeShapes() (NodeShapes, error) {
	return newNodeShapes(), nil
}

func (self *realActuator) GetDefaultNodeShape() (NodeShape, error) {
	return NodeShape{}, nil
}

func (self *realActuator) CreateNode(nodeShapeName string) (string, error) {
	return "", fmt.Errorf("unimplemented")
}

func New() Actuator {
	return &realActuator{}
}
