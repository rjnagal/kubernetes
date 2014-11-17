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
	return newNodeShapes(), nil
}

func (self *realActuator) GetDefaultNodeShape() (NodeShape, error) {
	return NodeShape{}, nil
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
