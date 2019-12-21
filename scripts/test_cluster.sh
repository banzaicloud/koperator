#!/bin/bash

# User needs at least the following installed
DEPS=("kubectl" "make" "docker")
# Make sure to pre-build and install helm from master - until 3.1 is available
HELM_VERSION="v3.0.1"
KIND_VERSION="v0.6.1"

# Manifests, tags, meta, etc.
K8S_CLUSTER_NAME="kafka"
NODE_IMAGE="kindest/node:${KUBERNETES_VERSION}"
JETSTACK_CHARTS="https://charts.jetstack.io"
BANZAI_CHARTS="https://kubernetes-charts.banzaicloud.com"
LB_MANIFEST="https://raw.githubusercontent.com/google/metallb/v0.8.1/manifests/metallb.yaml"
LB_CONFIG_MANIFEST="https://git.io/km-config.yaml"
CERT_MANAGER_CRDS="https://raw.githubusercontent.com/jetstack/cert-manager/release-0.11/deploy/manifests/00-crds.yaml"

# Path to kafka-operator resources - override cluster manifest with
# KAFKA_CLUSTER_MANIFEST.
REPO_DIR="$( readlink -nf "$( cd "$( dirname "${BASH_SOURCE[0]}" )" > /dev/null 2>&1 && pwd )/.." )"
KAFKA_CHART="${REPO_DIR}/charts/kafka-operator"

OS=$(uname | tr '[:upper:]' '[:lower:]')

# Creates a 3 worker - 1 master kubernetes cluster
create-cluster() {
  cat << EOF | kind create cluster --image ${NODE_IMAGE} --name "${K8S_CLUSTER_NAME}" --config -
kind: Cluster
apiVersion: kind.sigs.k8s.io/v1alpha3
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
EOF
}

# Creates a namespace against the local kind cluster
create-ns() {
  kubectl --context "kind-${K8S_CLUSTER_NAME}" create ns ${1}
}

# Runs arbitrary apply arguments against the local kind cluster
kubectl-apply() {
  kubectl --context "kind-${K8S_CLUSTER_NAME}" apply ${@}
}

# Runs arbitrary helm install arguments against the local kind cluster
helm-install() {
  helm install --kube-context "kind-${K8S_CLUSTER_NAME}" ${@}
}

# Sets up the required helm repos locally
configure-helm-repos() {
  helm repo add jetstack "${JETSTACK_CHARTS}"
  helm repo add banzaicloud "${BANZAI_CHARTS}"
  helm repo update
}

# Installs a load balancer implementation into the kind cluster
setup-loadbalancer() {
  kubectl-apply -f "${LB_MANIFEST}" -f "${LB_CONFIG_MANIFEST}"
}

# Installs cert-manager into the kind cluster
setup-certmanager() {
  create-ns cert-manager
  kubectl-apply --validate=false -f "${CERT_MANAGER_CRDS}"
  helm-install cert-manager jetstack/cert-manager --namespace cert-manager --version v0.11.0
}

# Installs zookeeper-operator and a 3 node instance into the kind cluster
setup-zookeeper() {
  create-ns zookeeper
  helm-install zookeeper-operator banzaicloud/zookeeper-operator --namespace zookeeper
  cat << EOF | kubectl apply -n zookeeper -f -
apiVersion: zookeeper.pravega.io/v1beta1
kind: ZookeeperCluster
metadata:
  name: example-zookeepercluster
  namespace: zookeeper
spec:
  replicas: 3
EOF
}

# Builds and installs the kafka-operator, then applies a cluster manifest
setup-kafka() {
  create-ns kafka
  make -C "${REPO_DIR}" docker-build
  kind load docker-image --name "${K8S_CLUSTER_NAME}" ${IMG}
  helm-install kafka-operator "${KAFKA_CHART}" \
    --namespace kafka \
    --set operator.image.repository=${IMG%%:*} \
    --set operator.image.tag=${IMG#*:} \
    --set operator.image.pullPolicy=IfNotPresent \
    --set operator.verboseLogging=true
  kubectl-apply -n kafka -f "${KAFKA_CLUSTER_MANIFEST}"
}

# Installs helm if not available
ensure-helm() {
  which helm 2> /dev/null && return
  echo "Helm is not installed, downloading now"
  curl -JLO https://get.helm.sh/helm-${HELM_VERSION}-${OS}-amd64.tar.gz && \
    tar xf helm-${HELM_VERSION}-${OS}-amd64.tar.gz && \
    echo "Installing helm, you may need to enter your password" && \
    chmod +x ${OS}-amd64/helm && sudo mv ${OS}-amd64/helm /usr/local/bin/helm && \
    rm -rf helm-${HELM_VERSION}-${OS}-amd64.tar.gz ${OS}-amd64/
}

# Installs kind if not available
ensure-kind() {
  which kind 2> /dev/null && return
  echo "Kind is not installed, downloading now"
  curl -JLO https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${OS}-amd64 && \
    echo "Installing kind, you may need to enter your password" && \
    chmod +x kind-${OS}-amd64 && sudo mv kind-${OS}-amd64 /usr/local/bin/kind
}

# Ensures all script dependencies
check-deps() {
  for exec in "${DEPS[@]}"; do
    echo -n "${exec} path - " && which ${exec}
  done
  ensure-kind
  ensure-helm
}

# Main function
main() {
  set -e
  check-deps
  create-cluster
  configure-helm-repos
  setup-loadbalancer
  setup-certmanager
  setup-zookeeper
  setup-kafka
}

main
