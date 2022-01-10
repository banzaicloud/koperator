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
/opt/kafka/bin/kafka-server-start.sh /config/broker-config
rm /var/run/wait/do-not-exit-yet
