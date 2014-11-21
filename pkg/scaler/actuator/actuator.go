package actuator

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/provisioner"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
)

type realActuator struct {
	serviceHostPort string
}

func (self *realActuator) GetNodeShapes() (NodeShapes, error) {
	var response map[string]api.NodeResources
	if err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/instance_types", self.serviceHostPort), nil, &response); err != nil {
		return NodeShapes{}, err
	}

	if len(response) == 0 {
		return NodeShapes{}, fmt.Errorf("no node shapes returned by actuator.")
	}
	nodeShapes := newNodeShapes()
	for shape, resources := range response {
		capacity := types.Resource{
			Cpu:    uint64(resources.Capacity["cpu"].IntVal),
			Memory: uint64(resources.Capacity["memory"].IntVal),
		}
		nodeShapes.add(capacity, shape)
	}

	return nodeShapes, nil
}

func (self *realActuator) GetDefaultNodeShape() (string, error) {
	var response string
	err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/instance_types/default", self.serviceHostPort), nil, &response)

	return response, err
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

func New(actuatorHostPort string) (Actuator, error) {
	if actuatorHostPort == "" {
		return nil, fmt.Errorf("actuator host port empty.")
	}
	if len(strings.Split(actuatorHostPort, ":")) != 2 {
		return nil, fmt.Errorf("actuator host port invalid - %s.", actuatorHostPort)
	}

	return &realActuator{actuatorHostPort}, nil
}
