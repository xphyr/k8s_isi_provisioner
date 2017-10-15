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
	"context"
	"errors"
	"flag"
	"os"
	"strings"

	isi "github.com/codedellemc/goisilon"
	"github.com/kubernetes-incubator/external-storage/lib/controller"
	"k8s.io/api/core/v1"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	provisionerName    = "github.com/xphyr"
	provisionerNameKey = "PROVISIONER_NAME"
)

type isilonProvisioner struct {
	// Kubernetes Client. Use to retrieve Ceph admin secret
	client kubernetes.Interface

	// Identity of this isilonProvisioner, set to node's name. Used to identify
	// "this" provisioner's PVs.
	identity  string
	server    string
	path      string
	isiClient *isi.Client
}

var _ controller.Provisioner = &isilonProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *isilonProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {

	// using the same naming convention from the nfs example, lets create a volume name
	pvcNamespace := options.PVC.Namespace
	pvcName := options.PVC.Name

	volumeName := strings.Join([]string{pvcNamespace, pvcName, options.PVName}, "-")
	// using the isilon client created, create a volume, then if it works
	// create a nfs share we can return
	volume, err := p.isiClient.CreateVolume(context.Background(), volumeName)

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				"isilonProvisionerIdentity": p.identity,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Server:   p.server,
					Path:     p.path,
					ReadOnly: false,
				},
			},
		},
	}

	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *isilonProvisioner) Delete(volume *v1.PersistentVolume) error {
	ann, ok := volume.Annotations["isilonProvisionerIdentity"]
	if !ok {
		return errors.New("identity annotation not found on PV")
	}
	if ann != p.identity {
		return &controller.IgnoredError{Reason: "identity annotation on PV does not match ours"}
	}

	if err := os.RemoveAll(volume.Name); err != nil {
		return err
	}

	return nil
}

var (
	master     = flag.String("master", "", "Master URL")
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig")
	id         = flag.String("id", "", "Unique provisioner identity")
)

func main() {

	flag.Parse()
	flag.Set("logtostderr", "true")

	server := os.Getenv("NFS_SERVER")
	if server == "" {
		glog.Fatal("NFS_SERVER not set")
	}
	path := os.Getenv("NFS_PATH")
	if path == "" {
		glog.Fatal("NFS_PATH not set")
	}

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

	// We need to make a connection to the isilon
	// This is just a test for now, we will need to get all this config
	ic, err := isi.NewClientWithArgs(
		context.Background(),
		"https://192.168.5.200:8080",
		true,
		"userName",
		"groupName",
		"password",
		"/ifs/volumes")
	if err != nil {
		glog.Fatalf("Error making connection to Isilon: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	glog.Infof("Creating Isilon provisioner %s with identity: %s", prName, prID)

	isilonProvisioner := &isilonProvisioner{
		server:    server,
		path:      path,
		identity:  prID,
		isiClient: ic,
	}

	// Start the provision controller which will dynamically provision isilon
	// PVs
	pc := controller.NewProvisionController(
		clientset,
		prName,
		isilonProvisioner,
		serverVersion.GitVersion,
	)

	pc.Run(wait.NeverStop)
}
