package main

import (
	"flag"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/scaler/core"
	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	autoScaler, err := core.New()
	if err != nil {
		glog.Fatal(err)
	}
	_ = autoScaler.AutoScale()
}
