package aggregator

import (
	"flag"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/types"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/statscollector"
)

type realAggregator struct {
	aggregatorAddress string
}

var argAggregatorAddress = flag.String("aggregator_address", "localhost:8085", "Aggregator Host:Port.")

func (self *realAggregator) GetClusterInfo() (map[string]Node, error) {
	var response map[string]statscollector.NodeData
	if err := types.PostRequestAndGetResponse(fmt.Sprintf("http://%s/stats", self.aggregatorAddress), nil, &response); err != nil {
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

func New() (Aggregator, error) {
	if *argAggregatorAddress == "" || len(strings.Split(*argAggregatorAddress, ":")) != 2 {
		return nil, fmt.Errorf("Arrgregator address invalid: %s", *argAggregatorAddress)
	}

	return &realAggregator{*argAggregatorAddress}, nil
}
