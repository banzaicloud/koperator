#!/usr/bin/env bash

version=2.0.0-rc.0 # latest stable version
arch=amd64
opsys=$(echo "$(uname -s)" | awk '{print tolower($0)}')

# download the release
curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_${opsys}_${arch}.tar.gz"

# extract the archive
tar -zxvf kubebuilder_${version}_${opsys}_${arch}.tar.gz
mv kubebuilder_${version}_${opsys}_${arch} kubebuilder && mkdir -p bin && mv kubebuilder bin/

# delete tar file
rm kubebuilder_${version}_${opsys}_${arch}.tar.gz
