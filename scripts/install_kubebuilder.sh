#!/usr/bin/env bash

version=3.0.0-beta.1 # latest stable version
arch=amd64
opsys=$(echo "$(uname -s)" | awk '{print tolower($0)}')

# download the release
curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_${opsys}_${arch}"

mv kubebuilder_${version}_${opsys}_${arch} kubebuilder && mkdir -p bin && mv kubebuilder bin/
