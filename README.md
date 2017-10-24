# k8s_isi_provisioner
[![Build Status](https://travis-ci.org/xphyr/k8s_isi_provisioner.svg?branch=master)](https://travis-ci.org/xphyr/k8s_isi_provisioner.svg?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/xphyr/k8s_isi_provisioner)](https://goreportcard.com/report/github.com/xphyr/k8s_isi_provisioner)

Kubernetes external storage provisioner for Dell Isilon

Based on the following:
https://github.com/kubernetes-incubator/external-storage
https://github.com/codedellemc/goisilon

Instructions:
In order to use this external provisioner, you will need to compile the code, as it is not published to dockerhub yet.
To do so ensure you have go, and glide as well as make installed.
To build the software, run make.

To deploy the provisioner, run 
oc create -f pod.yaml
Create a storage class using the class.yaml file 
oc create -f class.yaml

To create a persistent volume, create a pvc and add an annotaion:
volume.beta.kubernetes.io/storage-class: "k8s-isilon"
This will enable the automatic creation of a persistent volume.

Tested against: 
https://www.emc.com/products-solutions/trial-software-download/isilon.htm

This provisioner has support for Isilon Storage Quotas, however they have not been tested due to not having a license.

## Parameters
**Param**|**Description**|**Example**
:-----:|:-----:|:-----:
ISI\_SERVER|The DNS name (or IP address) of the Isilon to use | isilon.somedomain.com
ISI\_PATH|The root path for all exports to be created in| \/ifs\/ose\_exports 
ISI\_USER|The user to connect to the isilon as|admin
ISI\_PASS|Password for the user account|password
ISI\_GROUP|The default group to assign to the share|users
ISI\_QUOTA\_ENABLE|Enable the use of quotas.  Defaults to disabled. | FALSE or TRUE

## Thanks

Thanks to the developers of the external storage provisioner code and the docs that are making this possible to do.
Thanks to Dell EMC {Code} for the great Isilon library.

This is not sponsored by Dell EMC or Kubernetes Foundation. 