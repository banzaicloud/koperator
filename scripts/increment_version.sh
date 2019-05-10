#!/bin/bash

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
