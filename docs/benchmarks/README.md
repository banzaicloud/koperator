This Readme is intended to help to setup the environment for the Kafka Performance Test using Amazon PKE, GKE, EKS.

We are going to use Banzai Cloud CLI to create the cluster:
```
brew install banzaicloud/tap/banzai-cli
banzai login
```

### PKE
As a first step create your own VPC and subnets on Amazon Management Console.
- Please use the provided wizard and select VPC with Single Public Subnet. (Please remember the Availability Zone you chosed.)
- Save the used route table id on the generated subnet
- Create two additional subnet in the VPC (choose different Availability Zones)
    - Modify your newly created subnet Auto Assign IP setting
    - Enable auto-assign public IPV4 address
- Assign the saved route table id to the two additional subnets
    - On Route Table page click Actions and Edit subnet associations

Next step is to create the cluster itself.

```
banzai cluster create
```

The required cluster template file can be found [here](infrastructure/cluster_pke.json)

> Please don't forget to fill out the template with the created ids.

This will create a cluster with 3 nodes for ZK 3 for Kafka 1 Master node and 2 node for clients.

### GKE
Create the cluster itself:

```
banzai cluster create
```
The required cluster template file can be found [here](infrastructure/cluster_gke.json)

> Please don't forget to fill out the template with the created ids.

### EKS

Create the cluster itself:

```
banzai cluster create
```
The required cluster template file can be found [here](infrastructure/cluster_eks.json)

> Please don't forget to fill out the template with the created ids.

Once your cluster is up and running we can move on to set up the Kubernetes infrastructure.

First please create a StorageClass which enables high performance disk requests.

EKS/PKE:

```
kubectl create -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/aws-ebs
parameters:
  type: io1
  iopsPerGB: "50"
  fsType: ext4
volumeBindingMode: WaitForFirstConsumer
EOF
```

GKE:

```
kubectl create -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/gce-pd
parameters:
  type: pd-ssd
volumeBindingMode: WaitForFirstConsumer
EOF
```

Create a Zookeeper cluster with 3 replicas using Pravega's Zookeeper Operator.
```
helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com/
helm install --name zookeeper-operator --namespace zookeeper banzaicloud-stable/zookeeper-operator
kubectl create -f - <<EOF
apiVersion: zookeeper.pravega.io/v1beta1
kind: ZookeeperCluster
metadata:
  name: example-zookeepercluster
  namespace: zookeeper
spec:
  replicas: 1
EOF
```

Install the latest version of Banzai Cloud Kafka Operator
```
helm install --name=kafka-operator banzaicloud-stable/kafka-operator
```

Create a 3 broker Kafka Cluster using the [provided](infrastructure/kafka.yaml) yaml.

This will install 3 brokers partitioned to three different zone with fast ssd.

To create the topic we should create a client container inside the cluster.
```
kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  annotations:
    linkerd.io/inject: enabled
  name: kafka-test
spec:
  containers:
  - name: kafka-test
    image: "wurstmeister/kafka:2.12-2.1.1"
    # Just spin & wait forever
    command: [ "/bin/bash", "-c", "--" ]
    args: [ "while true; do sleep 3000; done;" ]
EOF
```

Exec into this client and create the `perftest, perftest2, perftes3` topics.
```
kubectl exec -it kafka-test bash
./opt/kafka/bin/kafka-topics.sh --zookeeper zookeeper-client.zookeeper:2181 --topic perftest --create --replication-factor 3 --partitions 3
./opt/kafka/bin/kafka-topics.sh --zookeeper zookeeper-client.zookeeper:2181 --topic perftest2 --create --replication-factor 3 --partitions 3
./opt/kafka/bin/kafka-topics.sh --zookeeper zookeeper-client.zookeeper:2181 --topic perftest3 --create --replication-factor 3 --partitions 3
```

Monitoring environment automatically installed, find your cluster and Grafanas UI/credentials on our [UI](https://beta.banzaicloud.io). To monitor the infrastructure we used the official Node Exporter dashboard available with id `1860`.

Run perf test against the cluster, by building the provided Docker [image](loadgens/Dockerfile)
```
docker build -t yourname/perfload:0.1.0 /loadgens
docker push yourname/perfload:0.1.0
```

Submit the perf test application:

```
kubectl create -f - <<EOF
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: loadtest
  annotations:
    linkerd.io/inject: enabled
  name: perf-load
  namespace: default
spec:
  progressDeadlineSeconds: 600
  replicas: 4
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: loadtest
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: loadtest
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: nodepool.banzaicloud.io/name
                operator: In
                values:
                - pool3
      containers:
      - args:
        - -brokers=kafka-0:29092,kafka-1:29092,kafka-2:29092
        - -topic=perftest
        - -required-acks=all
        - -message-size=512
        - -workers=20
        image: yourorg/yourimage:yourtag
        imagePullPolicy: Always
        name: sangrenel
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
---

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: loadtest
  name: perf-load
  namespace: default
spec:
  progressDeadlineSeconds: 600
  replicas: 4
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: loadtest
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: loadtest
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: nodepool.banzaicloud.io/name
                operator: In
                values:
                - clients
      containers:
      - args:
        - -brokers=kafka-0,kafka-1,kafka-2:29092
        - -topic=perftest2
        - -required-acks=all
        - -message-size=512
        - -workers=20
        image: yourorg/yourimage:yourtag
        imagePullPolicy: Always
        name: sangrenel
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
---

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: loadtest
  name: perf-load
  namespace: default
spec:
  progressDeadlineSeconds: 600
  replicas: 4
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: loadtest
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: loadtest
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: nodepool.banzaicloud.io/name
                operator: In
                values:
                - clients
      containers:
      - args:
        - -brokers=kafka-0,kafka-1,kafka-2:29092
        - -topic=perftest
        - -required-acks=all
        - -message-size=512
        - -workers=20
        image: yourorg/yourimage:yourtag
        imagePullPolicy: Always
        name: sangrenel
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
EOF
```
