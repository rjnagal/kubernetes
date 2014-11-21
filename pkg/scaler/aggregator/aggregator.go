package aggregator

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/statscollector"
)

type realAggregator struct {
	aggregatorHostPort string
}

func (self *realAggregator) GetClusterInfo() (map[string]Node, error) {
	var response map[string]statscollector.NodeData
	if err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/stats", self.aggregatorHostPort), nil, &response); err != nil {
		return map[string]Node{}, err
	}

	result := make(map[string]Node)
	for host, nodeData := range response {
		node := Node{
			Hostname: host,
			Capacity: types.Resource{Cpu: uint64(nodeData.Capacity.Cpu), Memory: nodeData.Capacity.Memory},
			Usage:    DerivedStats(nodeData.Stats),
		}
		result[host] = node
	}
	return result, nil
}

func New(aggregatorHostPort string) (Aggregator, error) {
	if aggregatorHostPort == "" || len(strings.Split(aggregatorHostPort, ":")) != 2 {
		return nil, fmt.Errorf("Arrgregator address invalid: %s", aggregatorHostPort)
	}

	return &realAggregator{aggregatorHostPort}, nil
}
