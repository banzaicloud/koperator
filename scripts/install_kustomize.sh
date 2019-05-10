#!/usr/bin/env bash

version=1.0.11 # versions starting from v2.0.0 have issues with outside paths: https://github.com/kubernetes-sigs/kustomize/issues/776
opsys=$(echo "$(uname -s)" | awk '{print tolower($0)}')

# download the release
curl -O -L https://github.com/kubernetes-sigs/kustomize/releases/download/v${version}/kustomize_${version}_${opsys}_amd64

# move to bin
mkdir -p bin
mv kustomize_*_${opsys}_amd64 bin/kustomize
chmod u+x bin/kustomize