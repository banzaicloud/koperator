#!/usr/bin/env bash

: ${1?"Usage: $0 PATCH_FILE IMG_NAME"}
: ${2?"Usage: $0 PATCH_FILE IMG_NAME"}

cat << EOF > ${1}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
  labels:
    control-plane: controller-manager
spec:
  template:
    spec:
      containers:
      # Change the value of image field below to your controller image URL
      - image: ${2}
        name: manager
EOF
