package core

import (
	"github.com/golang/glog"
)

func (self *realAutoScaler) updateNewNodes(cluster *Cluster) {
	for _, node := range cluster.Current {
		if _, ok := self.newNodes[node.Hostname]; ok {
			delete(self.newNodes, node.Hostname)
		}
	}
}

func (self *realAutoScaler) updateExistingNodes(cluster *Cluster) (newNodes, removedNodes uint) {
	for hostname, node := range cluster.Current {
		if _, ok := self.existingNodes[hostname]; !ok {
			self.existingNodes[hostname] = node
			newNodes++
		}
	}
	for hostname := range self.existingNodes {
		if _, ok := cluster.Current[hostname]; !ok {
			delete(self.existingNodes, hostname)
			removedNodes++
		}
	}
	return
}

func (self *realAutoScaler) needResizing(cluster *Cluster) (newNodes []string) {
	// If no historical data exists, cache current cluster data and create any new nodes if any.
	if len(self.existingNodes) == 0 {
		for _, node := range cluster.Current {
			self.existingNodes[node.Hostname] = node
		}
		newNodes = append(newNodes, cluster.New...)
		return
	}
	// If historical data exists the following scenarios needs to be considered.
	// 1. Cluster size remains the same:
	//   In this scenario, we could have some existing nodes removed and new nodes added too.
	//   The scaler need not react to such a change.
	//   a. Existing node creation pending: Skip this iteration.
	//   b. No pending node creations and new nodes needs to be added: Add new nodes immediately.
	// 2. Cluster size increases:
	//    New nodes can be added by other services or due to creations triggered by this service.
	//    Check if any pending node creations have succeeded.
	//    Check if any existing nodes have been removed.
	//    Cache any new nodes created.
	//   a. Existing node creation pending: Skip this iteration.
	//   b. No pending node creations and new nodes needs to be added: Add new nodes immediately.
	// 3. Cluster size reduces:
	//   This scenario is tricky. The number of nodes could have dropped either due to an explicit downsizing or due to node failure.
	//   In this scenario, if we have pending node creations, we can either,
	//    i. Wait till all pending node creations complete (or)
	//    ii. Create more nodes if the pending node creations do not suffice the current load.
	//   For the sake of simplicity the current version implements option 'i'.

	// Update pending node creations
	self.updateNewNodes(cluster)

	// Update the scalers view of existing nodes.
	// TODO(vishh): If the net effect is a reduction in cluster size then consider adding new nodes, if required, even if there are pending new node creations.
	_, _ = self.updateExistingNodes(cluster)

	// Some nodes are yet to become active. Let the cluster settle down before altering it.
	if len(self.newNodes) > 0 {
		// There is a chance that the newly created node became inactive or got removed by another service.
		// We should either have a timeout and give up on the pending nodes
		// or query an authoritative source for the state of the node.
		return
	}

	// Add new nodes.
	newNodes = append(newNodes, cluster.New...)

	return
}

func (self *realAutoScaler) handleClusterResizing(cluster *Cluster) error {
	newNodes := self.needResizing(cluster)
	// Create new nodes if needed.
	for _, shapeName := range newNodes {
		hostname, err := self.actuator.CreateNode(shapeName)
		if err != nil {
			glog.Errorf("Failed to create a new node - %q", err)
		} else {
			self.newNodes[hostname] = shapeName
		}
	}

	return nil
}
