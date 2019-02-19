#!/bin/sh
set -e

echo "Starting Envoy..."
/usr/local/bin/envoy -c /etc/envoy/envoy.yaml
