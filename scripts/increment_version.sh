#!/bin/bash
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


while getopts ":Mmp" opt; do
  case $opt in
    M )
      relType=major
      ;;
    m )
      relType=minor
      ;;
    p )
      relType=patch
      ;;
  esac
done

if [ -z $relType ]; then
  echo "usage: $(basename $0) [-Mmp] major.minor.patch"
  exit 1
fi

if [ -z $2 ]; then
  echo "usage: $(basename $0) [-Mmp] major.minor.patch"
  exit 1
fi

version=$2
parts=( ${version//./ } )

if [ $relType == "major" ]; then
  ((parts[0]++))
  parts[1]=0
  parts[2]=0
fi

if [ $relType == "minor" ]; then
  ((parts[1]++))
  parts[2]=0
fi

if [ $relType == "patch" ]; then
  ((parts[2]++))
fi

echo "${parts[0]}.${parts[1]}.${parts[2]}"
