package aggregator

import (
// import the aggregator API once its ready.
)

type realAggregator struct {
}

func (self *realAggregator) GetClusterInfo() (map[string]Node, error) {
	return map[string]Node{}, nil
}

func New() Aggregator {
	return &realAggregator{}
}
