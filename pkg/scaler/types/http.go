package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
)

func PostRequestAndGetResponse(url string, data, response interface{}) error {
	var resp *http.Response
	var err error

	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("unable to marshal data: %v", err)
		}
		resp, err = http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	} else {
		resp, err = http.Get(url)
	}
	if err != nil {
		err = fmt.Errorf("unable to get %v: %v", err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("unable to read all %v: %v", err)
		return err
	}
	glog.V(2).Infof("Url: %s, Request: %v, Response: %s", url, data, body)
	if err = json.Unmarshal(body, response); err != nil {
		err = fmt.Errorf("unable to unmarshal %v: %v", string(body), err)
		return err
	}
	return nil
}
