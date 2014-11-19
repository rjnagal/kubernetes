package actuator

import (
	"flag"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/provisioner"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
)

type realActuator struct {
	serviceHostPort string
}

var argActuatorHostPort = flag.String("actuator_hostport", "localhost:8080", "Actuator Host:Port.")

func (self *realActuator) GetNodeShapes() (NodeShapes, error) {
	var response []string
	if err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/instance_types", self.serviceHostPort), nil, &response); err != nil {
		return NodeShapes{}, err
	}

	if len(response) == 0 {
		return NodeShapes{}, fmt.Errorf("no node shapes returned by actuator.")
	}
	nodeShapes := newNodeShapes()
	for _, shape := range response {
		nodeShapes.add(NodeShape{Name: shape})
	}

	return nodeShapes, nil
}

func (self *realActuator) GetDefaultNodeShape() (NodeShape, error) {
	var response string
	if err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/instance_types/default", self.serviceHostPort), nil, &response); err != nil {
		return NodeShape{}, err
	}

	if response == "" {
		return NodeShape{}, fmt.Errorf("default node shape returned by actuator is empty.")
	}

	return NodeShape{Name: response}, nil
}

func (self *realActuator) CreateNode(nodeShapeName string) (string, error) {
	var request provisioner.AddInstancesRequest
	var response []provisioner.Instance
	request.InstanceTypes = []string{nodeShapeName}

	if err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/instances", self.serviceHostPort), request, &response); err != nil {
		return "", err
	}

	if len(response) != 1 {
		return "", fmt.Errorf("invalid response from the actuator - %v", response)
	}

	return response[0].Name, nil
}

func New() (Actuator, error) {
	if *argActuatorHostPort == "" {
		return nil, fmt.Errorf("actuator host port empty.")
	}
	if len(strings.Split(*argActuatorHostPort, ":")) != 2 {
		return nil, fmt.Errorf("actuator host port invalid - %s.", *argActuatorHostPort)
	}

	return &realActuator{*argActuatorHostPort}, nil
}
