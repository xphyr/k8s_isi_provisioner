/*
Copyright 2016 The Kubernetes Authors.

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
	"errors"
	"flag"
	"os"
	"path"
	"time"

	"syscall"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/external-storage/lib/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	isi "github.com/codedellemc/goisilon"

)

const (
	provisionerName           = "github.com/xphyr"
)

type isilonProvisioner struct {
	// Kubernetes Client. Use to retrieve Ceph admin secret
	client kubernetes.Interface

	// Identity of this hostPathProvisioner, set to node's name. Used to identify
	// "this" provisioner's PVs.
	identity string
}

// NewIsilonProvisioner creates a new Dell/EMC Isilon Provisioner
func NewIsilonProvisioner(client kubernetes.Interface, id string) controller.Provisioner {
	return &hostPathProvisioner{
		client: client,
		identity: id,
	}
}

var _ controller.Provisioner = &isilonProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *hostPathProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	path := path.Join(p.pvDir, options.PVName)

	if err := os.MkdirAll(path, 0777); err != nil {
		return nil, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				"hostPathProvisionerIdentity": p.identity,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
		},
	}

	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *hostPathProvisioner) Delete(volume *v1.PersistentVolume) error {
	ann, ok := volume.Annotations["hostPathProvisionerIdentity"]
	if !ok {
		return errors.New("identity annotation not found on PV")
	}
	if ann != p.identity {
		return &controller.IgnoredError{Reason: "identity annotation on PV does not match ours"}
	}

	path := path.Join(p.pvDir, volume.Name)
	if err := os.RemoveAll(path); err != nil {
		return err
	}

	return nil
}

var (
	master     = flag.String("master", "", "Master URL")
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig"
	id         = flag.String("id", "", "Unique provisioner identity")
)

func main() {

	flag.Parse()
	flag.Set("logtostderr", "true")

	var config *rest.Config
	var err error
	if *master != "" || *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}

	prName := provisionerName
	prNameFromEnv := os.Getenv(provisionerNameKey)
	if prNameFromEnv != "" {
		prName = prNameFromEnv
	}

	// By default, we use provisioner name as provisioner identity.
	// User may specify their own identity with `-id` flag to distinguish each
	// others, if they deploy more than one CephFS provisioners under same provisioner name.
	prID := prName
	if *id != "" {
		prID = *id
	}

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Error getting server version: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	glog.Infof("Creating Isilon provisioner %s with identity: %s", prName, prID)
	isilonFSProvisioner := newIsilonFSProvisioner(clientset, prID)

	// Start the provision controller which will dynamically provision hostPath
	// PVs
	pc := controller.NewProvisionController(
		clientset,
		prName,
		isilonFSProvisioner,
		serverVersion.GitVersion,
	)

	pc.Run(wait.NeverStop)
}
