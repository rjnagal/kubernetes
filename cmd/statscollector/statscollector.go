/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/statscollector"
	"github.com/golang/glog"
)

var address = flag.String("address", "0.0.0.0", "The IP address for serving stats.")
var port = flag.Int("port", 8085, "The port to listen on for connections.")
var fake = flag.Bool("fake", false, "Use fake services.")
var pollInterval = flag.Duration("poll_interval", 1*time.Minute, "Interval between polling stats for a node.")
var kubeMaster = flag.String("kubernetes_master_readonly", "", "IP for kubernetes master read-only API.")
var kubeletPort = flag.Int("kubelet_port", 10250, "Kubelet port")

func writeResult(res interface{}, w http.ResponseWriter) error {
	out, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshal response %+v with error: %s", res, err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
	return nil
}

func main() {
	flag.Parse()

	var clusterApi statscollector.Cluster
	var nodeApi statscollector.NodeApi
	var err error

	glog.Infof("Running with fake=%v on %s:%d", *fake, *address, *port)
	if *address == "" {
		glog.Fatal(fmt.Errorf("Need an address to serve stats requests."))
	}

	// TODO(jnagal): Remove fake. Move to tests.
	if *fake {
		// Create a fake cluster with 10 machines and a fake Node API.
		clusterApi, err = statscollector.NewFakeCluster(10)
		if err != nil {
			glog.Fatal(err)
		}
		nodeApi, err = statscollector.NewFakeNodeApi()
		if err != nil {
			glog.Fatal(err)
		}
	} else {
		clusterApi, err = statscollector.NewCluster(*kubeMaster)
		if err != nil {
			glog.Fatal(err)
		}
		nodeApi, err = statscollector.NewKubeNodeApi(*kubeletPort)
		if err != nil {
			glog.Fatal(err)
		}
	}
	statscollector, err := statscollector.New(nodeApi, clusterApi, *pollInterval)
	if err != nil {
		glog.Fatal(err)
	}

	err = statscollector.Start()
	if err != nil {
		glog.Fatal(err)
	}
	defer statscollector.Stop()

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		// TODO(jnagal): Add a request to return specific data.
		nodeData, err := statscollector.GetNodeStats()
		if err != nil {
			glog.Info("Failed to get data from statscollector: %s", err)
			http.Error(w, err.Error(), 500)
			return
		}
		err = writeResult(nodeData, w)
		if err != nil {
			glog.Errorf("Failed to write node data %+v", nodeData)
			http.Error(w, err.Error(), 500)
			return
		}
		return
	})

	glog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *address, *port), nil))
}
