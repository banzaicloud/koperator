#
# Copyright © 2022 Banzai Cloud
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
#
if [[ -n "$ENVOY_SIDECAR_STATUS" ]]; then
  COUNT=0
  MAXCOUNT=${1:-30}
  HEALTHYSTATUSCODE="200"
  while true; do
    COUNT=$(expr $COUNT + 1)
    SC=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:15000/ready)
    echo "waiting for envoy proxy to come up";
    sleep 1;
    if [[ "$SC" == "$HEALTHYSTATUSCODE" || "$MAXCOUNT" == "$COUNT" ]]; then
      break
    fi
  done
fi
touch /var/run/wait/do-not-exit-yet

# A few necessary steps if we are in KRaft mode
if [[ -n "${CLUSTER_ID}" ]]; then
  # If the storage is already formatted (e.g. broker restarts), the kafka-storage.sh will skip formatting for that storage
  # thus we can safely run the storage format command regardless if the storage has been formatted or not
  echo "Formatting KRaft storage with cluster ID ${CLUSTER_ID}"
  /opt/kafka/bin/kafka-storage.sh format --cluster-id "${CLUSTER_ID}" -c /config/broker-config

  # Adding or removing controller nodes to the Kafka cluster would trigger cluster rolling upgrade so all the nodes in the cluster are aware of the newly added/removed controllers.
  # When this happens, Kafka's local quorum state file would be outdated since it is static and the Kafka server can't be started with conflicting controllers info (compared to info stored in ConfigMap),
  # so we need to wipe out the local state files before starting the server so the information about the controller nodes is up-to-date with what is stored in ConfigMap
  # (Note: although we don't know if the server start-up is due to scaling up/down of the controller nodes, it is not harmful to remove the quorum state file before the server start-up process
  #  because the server will re-create the quorum state file after it starts up successfully)
  if [[ -n "${LOG_DIRS}" ]]; then
    IFS=',' read -ra LOGS <<< "${LOG_DIRS}"
    for LOG in "${LOGS[@]}"; do
      QUORUM_STATE_FILE="${LOG}/kafka/__cluster_metadata-0/quorum-state"
      if [ -f "${QUORUM_STATE_FILE}" ]; then
          echo "Removing quorum-state file from \"${QUORUM_STATE_FILE}\""
          rm -f "${QUORUM_STATE_FILE}"
      fi
    done
  fi
fi

/opt/kafka/bin/kafka-server-start.sh /config/broker-config
rm /var/run/wait/do-not-exit-yet
