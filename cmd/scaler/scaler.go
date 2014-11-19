package main

import (
	"flag"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler"
	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	autoScaler, err := scaler.New()
	if err != nil {
		glog.Fatal(err)
	}
	_ = autoScaler.AutoScale()
}
