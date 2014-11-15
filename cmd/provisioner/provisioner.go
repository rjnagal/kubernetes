package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/provisioner"
	"github.com/golang/glog"
)

var listenIp = flag.String("listen_ip", "", "The IP to listen on for connections")
var port = flag.Int("port", 8080, "The port to listen on for connections")

// Parse the request from the HTTP body.
func getAddInstancesRequest(body io.ReadCloser) (provisioner.AddInstancesRequest, error) {
	var request provisioner.AddInstancesRequest
	decoder := json.NewDecoder(body)
	err := decoder.Decode(&request)
	if err != nil && err != io.EOF {
		return provisioner.AddInstancesRequest{}, fmt.Errorf("unable to decode the json value: %s", err)
	}

	return request, nil
}

func writeResult(res interface{}, w http.ResponseWriter) error {
	out, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshall response %+v with error: %s", res, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
	return nil
}

func main() {
	flag.Parse()

	prov, err := provisioner.New()
	if err != nil {
		glog.Fatal(err)
	}

	http.HandleFunc("/instances", func(w http.ResponseWriter, r *http.Request) {
		request, err := getAddInstancesRequest(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		glog.Infof("Request to create %d instances: %v", len(request.InstanceTypes), request.InstanceTypes)
		newInstances, err := prov.AddInstances(request)
		if err != nil {
			// TODO(vmarmol): Write the created instances.
			http.Error(w, err.Error(), 500)
			return
		}

		writeResult(newInstances, w)
		return
	})

	glog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *listenIp, *port), nil))
}
