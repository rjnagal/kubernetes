package provisioner

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/gce"
	"github.com/golang/glog"
)

type Provisioner interface {
	// Add an instance of the specified type. In the case of an error, some instances may already have been created.
	// Those that are created will be returned alongside the error.
	AddInstances(request AddInstancesRequest) ([]Instance, error)

	// Gets a list of the types of instances available.
	InstanceTypes() ([]string, error)

	// Returns the "default" instance type.
	DefaultInstanceType() (string, error)
}

// TODO(vmarmol): This may need to be generic size and we chose the type according to what is available.
type AddInstancesRequest struct {
	// Types of instances to create.
	InstanceTypes []string `json:"instance_types,omitempty"`
}

type Instance struct {
	Name         string `json:"name,omitempty"`
	InstanceType string `json:"instance_type,omitempty"`
}

func New(defaultInstanceType string) (Provisioner, error) {
	// TODO(vmarmol): Make this generically for any provider.
	gce, err := gce_cloud.NewGCECloud()
	if err != nil {
		return nil, err
	}
	instances, valid := gce.Instances()
	if !valid {
		return nil, fmt.Errorf("instance requests are not valid for the current cloud provider")
	}

	// Verify that the default instance type exists
	instanceTypes, err := instances.InstanceTypes()
	if err != nil {
		return nil, err
	}
	if _, ok := instanceTypes[defaultInstanceType]; !ok {
		return nil, fmt.Errorf("default instance type %q is not a valid instance type")
	}

	return &prov{
		cloudProvider:       gce,
		instances:           instances,
		defaultInstanceType: defaultInstanceType,
	}, nil
}

type prov struct {
	cloudProvider       cloudprovider.Interface
	instances           cloudprovider.Instances
	defaultInstanceType string
}

func (self *prov) AddInstances(request AddInstancesRequest) ([]Instance, error) {
	// TODO(vmarmol): Improve this logic.
	// Get a instance ID, assume they are created sequencially.
	machs, err := self.instances.List("kubernetes-minion.+")
	if err != nil {
		return []Instance{}, err
	}
	instanceBase := len(machs) + 1

	// Add all requested instances
	newInstances := make([]Instance, 0, len(request.InstanceTypes))
	for i, instanceType := range request.InstanceTypes {
		instanceId := instanceBase + i
		instanceName := fmt.Sprintf("kubernetes-minion-%d", instanceId)
		instanceIpRange := fmt.Sprintf("10.244.%d.0/24", instanceId)
		glog.Infof("Adding instance %q with IP range %q", instanceName, instanceIpRange)
		err := self.instances.Add(instanceName, instanceIpRange, instanceType)
		if err != nil {
			return newInstances, err
		}

		newInstances = append(newInstances, Instance{
			Name:         instanceName,
			InstanceType: instanceType,
		})
	}

	return newInstances, nil
}

func (self *prov) InstanceTypes() ([]string, error) {
	instanceTypes, err := self.instances.InstanceTypes()
	if err != nil {
		return []string{}, err
	}

	// Get a list of just the type names.
	typeNames := make([]string, 0, len(instanceTypes))
	for name, _ := range instanceTypes {
		typeNames = append(typeNames, name)
	}

	return typeNames, nil
}

func (self *prov) DefaultInstanceType() (string, error) {
	return self.defaultInstanceType, nil
}
