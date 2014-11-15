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

package gce_cloud

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/goauth2/compute/serviceaccount"
	compute "code.google.com/p/google-api-go-client/compute/v1"
	container "code.google.com/p/google-api-go-client/container/v1beta1"
	"github.com/GoogleCloudPlatform/gcloud-golang/compute/metadata"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/resources"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
)

// GCECloud is an implementation of Interface, TCPLoadBalancer and Instances for Google Compute Engine.
type GCECloud struct {
	service          *compute.Service
	containerService *container.Service
	projectID        string
	zone             string
	instanceID       string

	// GCE image URL for new nodes.
	image string

	// Tag to place on all minions.
	minionTag string

	// Hostname of the master.
	master string
}

func init() {
	cloudprovider.RegisterCloudProvider("gce", func(config io.Reader) (cloudprovider.Interface, error) { return NewGCECloud() })
}

func getMetadata(url string) (string, error) {
	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("X-Google-Metadata-Request", "True")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getProjectAndZone() (string, string, error) {
	url := "http://metadata/computeMetadata/v1/instance/zone"
	result, err := getMetadata(url)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(result, "/")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("unexpected response: %s", result)
	}
	return parts[1], parts[3], nil
}

func getInstanceID() (string, error) {
	url := "http://metadata/computeMetadata/v1/instance/hostname"
	result, err := getMetadata(url)
	if err != nil {
		return "", err
	}
	parts := strings.Split(result, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("unexpected response: %s", result)
	}
	return parts[0], nil
}

// newGCECloud creates a new instance of GCECloud.
func NewGCECloud() (*GCECloud, error) {
	projectID, zone, err := getProjectAndZone()
	if err != nil {
		return nil, err
	}
	// TODO: if we want to use this on a machine that doesn't have the http://metadata server
	// e.g. on a user's machine (not VM) somewhere, we need to have an alternative for
	// instance id lookup.
	instanceID, err := getInstanceID()
	if err != nil {
		return nil, err
	}
	client, err := serviceaccount.NewClient(&serviceaccount.Options{})
	if err != nil {
		return nil, err
	}
	svc, err := compute.New(client)
	if err != nil {
		return nil, err
	}
	containerSvc, err := container.New(client)
	if err != nil {
		return nil, err
	}
	return &GCECloud{
		service:          svc,
		containerService: containerSvc,
		projectID:        projectID,
		zone:             zone,
		instanceID:       instanceID,
		// TODO(vmarmol): Make these flags.
		image:     fmt.Sprintf("https://clients6.google.com/compute/v1/projects/%s/global/images/%s", "google-containers", "container-vm-v20141016"),
		minionTag: "kubernetes-minion",
		master:    "kubernetes-master",
	}, nil
}

func (gce *GCECloud) Clusters() (cloudprovider.Clusters, bool) {
	return gce, true
}

// TCPLoadBalancer returns an implementation of TCPLoadBalancer for Google Compute Engine.
func (gce *GCECloud) TCPLoadBalancer() (cloudprovider.TCPLoadBalancer, bool) {
	return gce, true
}

// Instances returns an implementation of Instances for Google Compute Engine.
func (gce *GCECloud) Instances() (cloudprovider.Instances, bool) {
	return gce, true
}

// Zones returns an implementation of Zones for Google Compute Engine.
func (gce *GCECloud) Zones() (cloudprovider.Zones, bool) {
	return gce, true
}

func makeHostLink(projectID, zone, host string) string {
	host = canonicalizeInstanceName(host)
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s",
		projectID, zone, host)
}

// Session Affinity Type string
type GCEAffinityType string

const (
	// AffinityTypeNone - no session affinity.
	GCEAffinityTypeNone GCEAffinityType = "None"
	// AffinityTypeClientIP is the Client IP based.
	GCEAffinityTypeClientIP GCEAffinityType = "CLIENT_IP"
	// AffinityTypeClientIP is the Client IP based.
	GCEAffinityTypeClientIPProto GCEAffinityType = "CLIENT_IP_PROTO"
)

func (gce *GCECloud) makeTargetPool(name, region string, hosts []string, affinityType GCEAffinityType) (string, error) {
	var instances []string
	for _, host := range hosts {
		instances = append(instances, makeHostLink(gce.projectID, gce.zone, host))
	}
	pool := &compute.TargetPool{
		Name:            name,
		Instances:       instances,
		SessionAffinity: string(affinityType),
	}
	_, err := gce.service.TargetPools.Insert(gce.projectID, region, pool).Do()
	if err != nil {
		return "", err
	}
	link := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/%s/targetPools/%s", gce.projectID, region, name)
	return link, nil
}

func (gce *GCECloud) waitForGlobalOp(op *compute.Operation) error {
	pollOp := op
	for pollOp.Status != "DONE" {
		var err error
		time.Sleep(time.Second * 2)
		pollOp, err = gce.service.GlobalOperations.Get(gce.projectID, op.Name).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

func (gce *GCECloud) waitForRegionOp(op *compute.Operation, region string) error {
	pollOp := op
	for pollOp.Status != "DONE" {
		var err error
		time.Sleep(time.Second * 10)
		pollOp, err = gce.service.RegionOperations.Get(gce.projectID, region, op.Name).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

func (gce *GCECloud) waitForZoneOp(op *compute.Operation) error {
	pollOp := op
	for pollOp.Status != "DONE" {
		var err error
		time.Sleep(time.Second * 2)
		pollOp, err = gce.service.ZoneOperations.Get(gce.projectID, gce.zone, op.Name).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

// TCPLoadBalancerExists is an implementation of TCPLoadBalancer.TCPLoadBalancerExists.
func (gce *GCECloud) TCPLoadBalancerExists(name, region string) (bool, error) {
	_, err := gce.service.ForwardingRules.Get(gce.projectID, region, name).Do()
	return false, err
}

//translate from what K8s supports to what the cloud provider supports for session affinity.
func translateAffinityType(affinityType api.AffinityType) GCEAffinityType {
	switch affinityType {
	case api.AffinityTypeClientIP:
		return GCEAffinityTypeClientIP
	case api.AffinityTypeNone:
		return GCEAffinityTypeNone
	default:
		glog.Errorf("unexpected affinity type: %v", affinityType)
		return GCEAffinityTypeNone
	}
}

// CreateTCPLoadBalancer is an implementation of TCPLoadBalancer.CreateTCPLoadBalancer.
func (gce *GCECloud) CreateTCPLoadBalancer(name, region string, externalIP net.IP, port int, hosts []string, affinityType api.AffinityType) (net.IP, error) {
	pool, err := gce.makeTargetPool(name, region, hosts, translateAffinityType(affinityType))
	if err != nil {
		return nil, err
	}
	req := &compute.ForwardingRule{
		Name:       name,
		IPProtocol: "TCP",
		PortRange:  strconv.Itoa(port),
		Target:     pool,
	}
	if len(externalIP) > 0 {
		req.IPAddress = externalIP.String()
	}
	op, err := gce.service.ForwardingRules.Insert(gce.projectID, region, req).Do()
	if err != nil {
		return nil, err
	}
	err = gce.waitForRegionOp(op, region)
	if err != nil {
		return nil, err
	}
	fwd, err := gce.service.ForwardingRules.Get(gce.projectID, region, name).Do()
	if err != nil {
		return nil, err
	}
	return net.ParseIP(fwd.IPAddress), nil
}

// UpdateTCPLoadBalancer is an implementation of TCPLoadBalancer.UpdateTCPLoadBalancer.
func (gce *GCECloud) UpdateTCPLoadBalancer(name, region string, hosts []string) error {
	var refs []*compute.InstanceReference
	for _, host := range hosts {
		refs = append(refs, &compute.InstanceReference{host})
	}
	req := &compute.TargetPoolsAddInstanceRequest{
		Instances: refs,
	}

	_, err := gce.service.TargetPools.AddInstance(gce.projectID, region, name, req).Do()
	return err
}

// DeleteTCPLoadBalancer is an implementation of TCPLoadBalancer.DeleteTCPLoadBalancer.
func (gce *GCECloud) DeleteTCPLoadBalancer(name, region string) error {
	_, err := gce.service.ForwardingRules.Delete(gce.projectID, region, name).Do()
	if err != nil {
		return err
	}
	_, err = gce.service.TargetPools.Delete(gce.projectID, region, name).Do()
	return err
}

// Take a GCE instance 'hostname' and break it down to something that can be fed
// to the GCE API client library.  Basically this means reducing 'kubernetes-
// minion-2.c.my-proj.internal' to 'kubernetes-minion-2' if necessary.
func canonicalizeInstanceName(name string) string {
	ix := strings.Index(name, ".")
	if ix != -1 {
		name = name[:ix]
	}
	return name
}

// IPAddress is an implementation of Instances.IPAddress.
func (gce *GCECloud) IPAddress(instance string) (net.IP, error) {
	instance = canonicalizeInstanceName(instance)
	res, err := gce.service.Instances.Get(gce.projectID, gce.zone, instance).Do()
	if err != nil {
		glog.Errorf("Failed to retrieve TargetInstance resource for instance:%s", instance)
		return nil, err
	}
	ip := net.ParseIP(res.NetworkInterfaces[0].AccessConfigs[0].NatIP)
	if ip == nil {
		return nil, fmt.Errorf("invalid network IP: %s", res.NetworkInterfaces[0].AccessConfigs[0].NatIP)
	}
	return ip, nil
}

// fqdnSuffix is hacky function to compute the delta between hostame and hostname -f.
func fqdnSuffix() (string, error) {
	fullHostname, err := exec.Command("hostname", "-f").Output()
	if err != nil {
		return "", err
	}
	hostname, err := exec.Command("hostname").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(fullHostname)[len(string(hostname)):]), nil
}

// List is an implementation of Instances.List.
func (gce *GCECloud) List(filter string) ([]string, error) {
	// GCE gives names without their fqdn suffix, so get that here for appending.
	// This is needed because the kubelet looks for its jobs in /registry/hosts/<fqdn>/pods
	// We should really just replace this convention, with a negotiated naming protocol for kubelet's
	// to register with the master.
	suffix, err := fqdnSuffix()
	if err != nil {
		return []string{}, err
	}
	if len(suffix) > 0 {
		suffix = "." + suffix
	}
	listCall := gce.service.Instances.List(gce.projectID, gce.zone)
	if len(filter) > 0 {
		listCall = listCall.Filter("name eq " + filter)
	}
	res, err := listCall.Do()
	if err != nil {
		return nil, err
	}
	var instances []string
	for _, instance := range res.Items {
		instances = append(instances, instance.Name+suffix)
	}
	return instances, nil
}

func makeResources(cpu float32, memory float32) *api.NodeResources {
	return &api.NodeResources{
		Capacity: api.ResourceList{
			resources.CPU:    util.NewIntOrStringFromInt(int(cpu * 1000)),
			resources.Memory: util.NewIntOrStringFromInt(int(memory * 1024 * 1024 * 1024)),
		},
	}
}

func canonicalizeMachineType(machineType string) string {
	ix := strings.LastIndex(machineType, "/")
	return machineType[ix+1:]
}

func (gce *GCECloud) GetNodeResources(name string) (*api.NodeResources, error) {
	instance := canonicalizeInstanceName(name)
	instanceCall := gce.service.Instances.Get(gce.projectID, gce.zone, instance)
	res, err := instanceCall.Do()
	if err != nil {
		return nil, err
	}

	// Get existing instance types.
	instanceTypes, err := gce.InstanceTypes()
	if err != nil {
		return nil, err
	}

	if res, ok := instanceTypes[canonicalizeMachineType(res.MachineType)]; ok {
		return &res, nil
	}

	glog.Errorf("unknown machine: %s", res.MachineType)
	return nil, nil
}

// TODO(vmarmol): Reuse the existing script..
var metadataConfig = `
#! /bin/bash
MASTER_NAME='%s'
MINION_IP_RANGE='%s'

download-or-bust() {
  until [[ -e "${1##*/}" ]]; do
    echo "Downloading binary release tar"
    curl --ipv4 -LO --connect-timeout 20 --retry 6 --retry-delay 10 "$1"
  done
}

install-salt() {
  apt-get update

  mkdir -p /var/cache/salt-install
  cd /var/cache/salt-install

  TARS=(
    libzmq3_3.2.3+dfsg-1~bpo70~dst+1_amd64.deb
    python-zmq_13.1.0-1~bpo70~dst+1_amd64.deb
    salt-common_2014.1.13+ds-1~bpo70+1_all.deb
    salt-minion_2014.1.13+ds-1~bpo70+1_all.deb
  )
  if [[ ${1-} == '--master' ]]; then
    TARS+=(salt-master_2014.1.13+ds-1~bpo70+1_all.deb)
  fi
  URL_BASE="https://storage.googleapis.com/kubernetes-release/salt"

  for tar in "${TARS[@]}"; do
    download-or-bust "${URL_BASE}/${tar}"
    dpkg -i "${tar}"
  done

  # This will install any of the unmet dependencies from above.
  apt-get install -f -y

}

# The repositories are really slow and there are GCE mirrors
sed -i -e "\|^deb.*http://http.debian.net/debian| s/^/#/" /etc/apt/sources.list
sed -i -e "\|^deb.*http://ftp.debian.org/debian| s/^/#/" /etc/apt/sources.list.d/backports.list

# Prepopulate the name of the Master
mkdir -p /etc/salt/minion.d
echo "master: $MASTER_NAME" > /etc/salt/minion.d/master.conf

cat <<EOF >/etc/salt/minion.d/log-level-debug.conf
log_level: debug
log_level_logfile: debug
EOF

# Our minions will have a pool role to distinguish them from the master.
cat <<EOF >/etc/salt/minion.d/grains.conf
grains:
  roles:
    - kubernetes-pool
  cbr-cidr: $MINION_IP_RANGE
  cloud: gce
EOF

install-salt

# Wait a few minutes and trigger another Salt run to better recover from
# any transient errors.
echo "Sleeping 180"
sleep 180
salt-call state.highstate || true
`

func (gce *GCECloud) Add(name, ipRange, instanceType string) error {
	// Verify instance type.
	instanceTypes, err := gce.InstanceTypes()
	if err != nil {
		return err
	}
	if _, ok := instanceTypes[instanceType]; !ok {
		return fmt.Errorf("unknown instance type %q", instanceType)
	}

	// Write config file.
	startupScript := fmt.Sprintf(metadataConfig, gce.master, ipRange)

	// Add firewall.
	network := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", gce.projectID, "default")
	firewall := compute.Firewall{
		Name:         fmt.Sprintf("%s-all", name),
		Network:      network,
		SourceRanges: []string{ipRange},
	}

	// Add all the allowed protocols to the firewall.
	for _, protocol := range []string{"tcp", "udp", "icmp", "esp", "ah", "sctp"} {
		firewall.Allowed = append(firewall.Allowed, &compute.FirewallAllowed{
			IPProtocol: protocol,
		})
	}
	firewallCall := gce.service.Firewalls.Insert(gce.projectID, &firewall)
	firewallOp, err := firewallCall.Do()
	if err != nil {
		return fmt.Errorf("failed to add firewall with error: %v", err)
	}
	err = gce.waitForGlobalOp(firewallOp)
	if err != nil {
		return fmt.Errorf("failed to add firewall while waiting for completion with error: %v", err)
	}

	newDisk := compute.Disk{
		Name: name,
		// TODO(vmarmol): Make disk size configurable.
		SizeGb: 10,
		Type:   fmt.Sprintf("https://clients6.google.com/compute/v1/projects/%s/zones/%s/diskTypes/pd-standard", gce.projectID, gce.zone),
	}
	diskCall := gce.service.Disks.Insert(gce.projectID, gce.zone, &newDisk)
	diskCall.SourceImage(gce.image)
	diskOp, err := diskCall.Do()
	if err != nil {
		return fmt.Errorf("failed to add disk with error: %v", err)
	}
	err = gce.waitForZoneOp(diskOp)
	if err != nil {
		return fmt.Errorf("failed to add disk while waiting for completion with error: %v", err)
	}

	// Add instance.
	serviceAccount, err := metadata.Get("instance/service-accounts/default/email")
	if err != nil {
		return err
	}
	newInstance := compute.Instance{
		CanIpForward: true,
		Disks: []*compute.AttachedDisk{
			&compute.AttachedDisk{
				Boot:       true,
				DeviceName: name,
				Mode:       "READ_WRITE",
				Source:     fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/disks/%s", gce.projectID, gce.zone, name),
				Type:       "PERSISTENT",
			},
		},
		MachineType: fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/machineTypes/%s", gce.projectID, gce.zone, instanceType),
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				&compute.MetadataItems{
					Key:   "startup-script",
					Value: startupScript,
				},
			},
		},
		Name: name,
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
						Name: "External NAT",
						Type: "ONE_TO_ONE_NAT",
					},
				},
				Network: network,
			},
		},
		Scheduling: &compute.Scheduling{
			AutomaticRestart:  true,
			OnHostMaintenance: "MIGRATE",
		},
		ServiceAccounts: []*compute.ServiceAccount{
			&compute.ServiceAccount{
				Email:  serviceAccount,
				Scopes: []string{"https://www.googleapis.com/auth/compute"},
			},
		},
		Tags: &compute.Tags{
			Items: []string{gce.minionTag},
		},
	}
	instanceCall := gce.service.Instances.Insert(gce.projectID, gce.zone, &newInstance)
	instanceOp, err := instanceCall.Do()
	if err != nil {
		return fmt.Errorf("failed to add instance with error: %v", err)
	}
	err = gce.waitForZoneOp(instanceOp)
	if err != nil {
		return fmt.Errorf("failed to add instance while waiting for completion with error: %v", err)
	}

	newRoute := compute.Route{
		Name:            name,
		Network:         network,
		DestRange:       ipRange,
		Priority:        1000,
		NextHopInstance: fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s", gce.projectID, gce.zone, name),
	}
	routeCall := gce.service.Routes.Insert(gce.projectID, &newRoute)
	routeOp, err := routeCall.Do()
	if err != nil {
		return fmt.Errorf("failed to add route with error: %v", err)
	}
	err = gce.waitForGlobalOp(routeOp)
	if err != nil {
		return fmt.Errorf("failed to add route while waiting for completion with error: %v", err)
	}

	return nil
}

func (gce *GCECloud) InstanceTypes() (map[string]api.NodeResources, error) {
	// TODO: Get this dynamically.
	return map[string]api.NodeResources{
		"f1-micro":       *makeResources(1, 0.6),
		"g1-small":       *makeResources(1, 1.70),
		"n1-standard-1":  *makeResources(1, 3.75),
		"n1-standard-2":  *makeResources(2, 7.5),
		"n1-standard-4":  *makeResources(4, 15),
		"n1-standard-8":  *makeResources(8, 30),
		"n1-standard-16": *makeResources(16, 60),
	}, nil
}

func (gce *GCECloud) GetZone() (cloudprovider.Zone, error) {
	region, err := getGceRegion(gce.zone)
	if err != nil {
		return cloudprovider.Zone{}, err
	}
	return cloudprovider.Zone{
		FailureDomain: gce.zone,
		Region:        region,
	}, nil
}

func (gce *GCECloud) AttachDisk(diskName string, readOnly bool) error {
	disk, err := gce.getDisk(diskName)
	if err != nil {
		return err
	}
	readWrite := "READ_WRITE"
	if readOnly {
		readWrite = "READ_ONLY"
	}
	attachedDisk := gce.convertDiskToAttachedDisk(disk, readWrite)
	_, err = gce.service.Instances.AttachDisk(gce.projectID, gce.zone, gce.instanceID, attachedDisk).Do()
	if err != nil {
		// Check if the disk is already attached to this instance.  We do this only
		// in the error case, since it is expected to be exceptional.
		instance, err := gce.service.Instances.Get(gce.projectID, gce.zone, gce.instanceID).Do()
		if err != nil {
			return err
		}
		for _, disk := range instance.Disks {
			if disk.InitializeParams.DiskName == diskName {
				// Disk is already attached, we're good to go.
				return nil
			}
		}

	}
	return err
}

func (gce *GCECloud) DetachDisk(devicePath string) error {
	_, err := gce.service.Instances.DetachDisk(gce.projectID, gce.zone, gce.instanceID, devicePath).Do()
	return err
}

func (gce *GCECloud) getDisk(diskName string) (*compute.Disk, error) {
	return gce.service.Disks.Get(gce.projectID, gce.zone, diskName).Do()
}

// getGceRegion returns region of the gce zone. Zone names
// are of the form: ${region-name}-${ix}.
// For example "us-central1-b" has a region of "us-central1".
// So we look for the last '-' and trim to just before that.
func getGceRegion(zone string) (string, error) {
	ix := strings.LastIndex(zone, "-")
	if ix == -1 {
		return "", fmt.Errorf("unexpected zone: %s", zone)
	}
	return zone[:ix], nil
}

// Converts a Disk resource to an AttachedDisk resource.
func (gce *GCECloud) convertDiskToAttachedDisk(disk *compute.Disk, readWrite string) *compute.AttachedDisk {
	return &compute.AttachedDisk{
		DeviceName: disk.Name,
		Kind:       disk.Kind,
		Mode:       readWrite,
		Source:     "https://" + path.Join("www.googleapis.com/compute/v1/projects/", gce.projectID, "zones", gce.zone, "disks", disk.Name),
		Type:       "PERSISTENT",
	}
}

func (gce *GCECloud) ListClusters() ([]string, error) {
	list, err := gce.containerService.Projects.Clusters.List(gce.projectID).Do()
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, cluster := range list.Clusters {
		result = append(result, cluster.Name)
	}
	return result, nil
}

func (gce *GCECloud) Master(clusterName string) (string, error) {
	return "k8s-" + clusterName + "-master.internal", nil
}
