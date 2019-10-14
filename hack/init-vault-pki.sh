#!/bin/bash

main() {
  if [[ -z "${VAULT_ADDR}" ]] || [[ -z "${VAULT_TOKEN}" ]]; then
    echo "You must set at least set VAULT_ADDR and VAULT_TOKEN in your environment"
    return
  fi
  init-auth-role
  init-pki
  init-user-store
}

init-auth-role() {
  # Kubernetes auth role for the operator
  cat << EOF | vault policy write kafka-operator -
path "pki_kafka/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "secret/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "kafka_users/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
EOF
  vault write auth/kubernetes/role/kafka-operator \
    bound_service_account_names=kafka-operator-operator \
    bound_service_account_namespaces=kafka \
    policies=kafka-operator
}

init-pki() {
  # Mount a PKI at pki_kafka
  vault secrets list | grep pki_kafka || vault secrets enable -path pki_kafka pki
  # Set the max_lease time to a year
  vault secrets tune -max-lease-ttl=8760h pki_kafka
  # Generate a CA certificate
  vault write pki_kafka/root/generate/internal \
    common_name=kafkaca.kafka.svc.cluster.local \
    ttl=8760h
  # Write distribution points
  vault write pki_kafka/config/urls \
    issuing_certificates="${VAULT_ADDR}/v1/pki_kafka/ca" \
    crl_distribution_points="${VAULT_ADDR}/v1/pki_kafka/crl"
  # Set up role for issuing certificates
  vault write pki_kafka/roles/operator \
    allow_localhost=true \
    allowed_domains="*" \
    allow_subdomains=true \
    max_ttl=8760h \
    allow_any_name=true \
    allow_ip_sans=true \
    allow_glob_domains=true \
    usr_csr_common_name=false \
    use_csr_sans=false
}

init-user-store() {
  vault secrets list | grep kafka_users || vault secrets enable -path kafka_users kv
}

# you can source this script instead to run the functions by themselves
# e.g. for testing only the auth role present but no CA setup.
if [[ "${0}" == "init-vault-pki.sh" ]] ; then
  main
fi
