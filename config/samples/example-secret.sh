#!/bin/bash

(
    cd cfssl
    ./gencert.sh

    FILE_ARGS=()

    for i in ca server peer client; do
      cp "${i}.pem" "${i}Cert"
      cp "${i}-key.pem" "${i}Key"
      FILE_ARGS+=(--from-file "${i}Cert" --from-file "${i}Key")
    done

    kubectl create secret generic test-kafka-operator -n kafka "${FILE_ARGS[@]}" --dry-run -o yaml
)
