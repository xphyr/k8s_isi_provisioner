# k8s_isi_provisioner
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

Tested against: 
https://www.emc.com/products-solutions/trial-software-download/isilon.htm

This provisioner has support for Isilon Storage Quotas, however they have not been tested due to not having a license.

Thanks to the developers of the external storage provisioner code and the docs that are making this possible to do.
Thanks to Dell EMC {Code} for the great Isilon library.

This is not sponsored by Dell EMC or Kubernetes Foundation. 