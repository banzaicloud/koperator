#!/usr/bin/env bash
# Copyright 2023 Cisco Systems, Inc. and/or its affiliates
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


version=3.1.0
opsys=$(echo "$(uname -s)" | awk '{print tolower($0)}')

# download the release
curl -O -L https://github.com/kubernetes-sigs/kustomize/releases/download/v${version}/kustomize_${version}_${opsys}_amd64
# move to bin
mkdir -p bin
mv kustomize_*_${opsys}_amd64 bin/kustomize
chmod u+x bin/kustomize
