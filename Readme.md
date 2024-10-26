#### Application Details

I have written two Apps in GoFiber. One is `client` server exposing the rabbitMQ setup and the other is `worker` consuming the messages in the queue.
Both the apps have been configured with a CI Pipeline located at `.github/workflows` with secrets that need to be added on the github repository's secrets.

- DOCKERHUB_USERNAME
- DOCKERHUB_TOKEN

Change the username `maniraja1122` based on your credentials in the workflow files.

Both these services have the environment variables, that need to be configured in the k8s deployment to connect to the RabbitMQ Cluster

- USER_RM
- PASSWORD_RM
- SVC_RM

We will use the RabbitMQ Addon to deploy a RabbitMQ Cluster that in this example.

##### RabbitMQ Client

Exposes port 8080 to send messages to queue.

```
http://<WORKER_K8S_SVC>:<WORKER_PORT>/send?msg=<YOUR TEXT HERE>
```

##### RabbitMQ Worker

Consumes messages from the Queue.

Create k8s cluster with autoscaling

```
eksctl create cluster --name test-cluster --region us-west-2 --nodes 2 --nodes-min 1 --nodes-max 3 --node-type t3.micro --asg-access
```

`--asg-access` flag is need for Cluster Autoscaler to work.

#### K8s Resources Details

##### Setup RabbitMQ Cluster

Install RabbitMQ:

```
kubectl apply -f "https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml"
```

[REQUIRED] Add Local Provisioner for StorageClass:

```
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml
kubectl annotate storageclass local-path storageclass.kubernetes.io/is-default-class=true
```

Deploy Sample Cluster:

```
kubectl apply -f https://raw.githubusercontent.com/rabbitmq/cluster-operator/main/docs/examples/hello-world/rabbitmq.yaml
```

Get the required variables for the client/worker env variables and replace that in the `k8s/client.yaml` and `k8s/worker.yaml`

```
username="$(kubectl get secret hello-world-default-user -o jsonpath='{.data.username}' | base64 --decode)"
password="$(kubectl get secret hello-world-default-user -o jsonpath='{.data.password}' | base64 --decode)"
service="$(kubectl get service hello-world -o jsonpath='{.spec.clusterIP}')"
```

##### RabbitMQ Client/Worker

Apply `k8s/client.yaml` and `k8s/worker.yaml`

#### Configure k8s Addons

##### CA (Cluster Autoscaler)

```

eksctl utils associate-iam-oidc-provider \
 --cluster test-cluster \
 --approve

```

```

cat <<EoF > k8s-asg-policy.json
{
"Version": "2012-10-17",
"Statement": [
{
"Action": [
"autoscaling:DescribeAutoScalingGroups",
"autoscaling:DescribeAutoScalingInstances",
"autoscaling:DescribeLaunchConfigurations",
"autoscaling:DescribeTags",
"autoscaling:SetDesiredCapacity",
"autoscaling:TerminateInstanceInAutoScalingGroup",
"ec2:DescribeLaunchTemplateVersions"
],
"Resource": "\*",
"Effect": "Allow"
}
]
}
EoF

aws iam create-policy \
 --policy-name k8s-asg-policy \
 --policy-document file://k8s-asg-policy.json

```

Replace ACCOUNT_ID with your AWS account ID.

```

eksctl create iamserviceaccount \
 --name cluster-autoscaler \
 --namespace kube-system \
 --cluster test-cluster \
 --attach-policy-arn "arn:aws:iam::${ACCOUNT_ID}:policy/k8s-asg-policy" \
 --approve \
 --override-existing-serviceaccounts

```

```

kubectl apply -f https://www.eksworkshop.com/beginner/080_scaling/deploy_ca.files/cluster-autoscaler-autodiscover.yaml

```

```
kubectl -n kube-system \
 annotate deployment.apps/cluster-autoscaler \
 cluster-autoscaler.kubernetes.io/safe-to-evict="false"
```

Edit the cluster autoscaler `deployment` with this argument in the container

```
--node-group-auto-discovery=asg:tag=k8s.io/cluster-autoscaler/enabled,k8s.io/cluster-autoscaler/test-cluster
```

##### HPA (Horizontal Pod Autoscaling)

Configure HPA for RabbitMQ, Client and Worker Deployments/Statefulsets

[Prerequisite] Install Metric Server

```
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
helm install metrics-server metrics-server/metrics-server
```

```
kubectl autoscale deployment rabbitmq-worker --cpu-percent=80 --min=1 --max=10
kubectl autoscale deployment rabbitmq-client --cpu-percent=80 --min=1 --max=10
kubectl autoscale statefulset hello-world-server --cpu-percent=80 --min=1 --max=10
```

#### TEST

##### Sample Request

Test Request:

```
http://<CLIENT_SVC_IP>:8080/send?msg="test utorify"
```

On worker logs check:

```
kubectl logs <WORKER_POD_NAME>
```

Sample Output:

```
2024/10/26 08:01:03 Successfully connected to RabbitMQ instance
2024/10/26 08:01:03 [*] - Waiting for messages
2024/10/26 08:01:36 Received message: "test utorify"
```

#### Working

##### CA

The Cluster AutoScaler will increase/decrease no of nodes based on resources demand.

Check Logs:

```
k logs <CA_POD> -n kube-system
```

Sample Output

```
I1026 08:24:09.142747       1 scale_down.go:447] Node ip-192-168-15-167.us-west-2.compute.internal - cpu utilization 0.181347
I1026 08:24:09.142778       1 scale_down.go:447] Node ip-192-168-41-11.us-west-2.compute.internal - cpu utilization 0.077720
I1026 08:24:09.142800       1 scale_down.go:447] Node ip-192-168-48-100.us-west-2.compute.internal - memory utilization 0.151108
I1026 08:24:09.142819       1 scale_down.go:443] Node ip-192-168-80-118.us-west-2.compute.internal is not suitable for removal - memory utilization too big (0.618937)
I1026 08:24:09.142839       1 scale_down.go:447] Node ip-192-168-86-230.us-west-2.compute.internal - cpu utilization 0.181347
I1026 08:24:09.142923       1 scale_down.go:564] Finding additional 4 candidates for scale down.
I1026 08:24:19.157947       1 static_autoscaler.go:502] Scale down status: unneededOnly=true lastScaleUpTime=2024-10-26 08:23:58.957179714 +0000 UTC m=+52.010210523 lastScaleDownDeleteTime=2024-10-26 08:23:48.956521164 +0000 UTC m=+42.009551943 lastScaleDownFailTime=2024-10-26 08:23:48.956521239 +0000 UTC m=+42.009552019 scaleDownForbidden=true isDeleteInProgress=false scaleDownInCooldown=true
```

The no of nodes will increase/decrease based on load.

Run:

```
k get nodes
```

Sample Output:

```
NAME                                           STATUS   ROLES    AGE     VERSION
ip-192-168-15-167.us-west-2.compute.internal   Ready    <none>   116m    v1.30.4-eks-a737599
ip-192-168-15-214.us-west-2.compute.internal   Ready    <none>   56m     v1.30.4-eks-a737599
ip-192-168-15-250.us-west-2.compute.internal   Ready    <none>   9m42s   v1.30.4-eks-a737599
ip-192-168-41-11.us-west-2.compute.internal    Ready    <none>   110m    v1.30.4-eks-a737599
ip-192-168-48-100.us-west-2.compute.internal   Ready    <none>   110m    v1.30.4-eks-a737599
ip-192-168-80-118.us-west-2.compute.internal   Ready    <none>   110m    v1.30.4-eks-a737599
ip-192-168-86-230.us-west-2.compute.internal   Ready    <none>   118m    v1.30.4-eks-a737599
```

We can see that some nodes are recently created based on load.

Note: The CA can't exceed the max-nodes count in the ASG(Autoscaling Group) so make sure to set that based on need. (In our case, it's 7 and it has hit the limit. We need to change the max-nodes in the ASG to add more nodes)

Also even the workload on the Pods is 0% but the nodes are created as there is limit on `max no of pods on a single node`.

##### HPA

The HPA will scale the no of replicas based on workload

Run:

```
k get hpa
```

Sample Output:

```
NAME                 REFERENCE                        TARGETS       MINPODS   MAXPODS   REPLICAS   AGE
hello-world-server   StatefulSet/hello-world-server   cpu: 0%/80%   1         10        1          61m
rabbitmq-client      Deployment/rabbitmq-client       cpu: 0%/80%   1         10        3          61m
rabbitmq-worker      Deployment/rabbitmq-worker       cpu: 0%/80%   1         10        3          61m
```

The `0%` shows no workload on the pods right now.

As there was no workload so `Replicas` for client/server changed to 1 (min value).

```
NAME                 REFERENCE                        TARGETS       MINPODS   MAXPODS   REPLICAS   AGE
hello-world-server   StatefulSet/hello-world-server   cpu: 0%/80%   1         10        1          62m
rabbitmq-client      Deployment/rabbitmq-client       cpu: 0%/80%   1         10        1          62m
rabbitmq-worker      Deployment/rabbitmq-worker       cpu: 0%/80%   1         10        1          62m
```

##### Put Workload

Note: Make sure to use a separate terminal for the curl command as that will run in a loop and watch the hpa stats in another terminal.
Now do some stress testing:
This send total 200 requests with max 10 in parallel.

```
for ((i=0; i<1000; ++i)); do
    seq 1 200 | xargs -I $ -n1 -P10  curl "http://a9935dc85d16e4c37a458d0279ddc55b-828238954.us-west-2.elb.amazonaws.com:8080/send?msg=test%20utorify"
done
```

Sample Output for `k get hpa`:

```
NAME                 REFERENCE                        TARGETS       MINPODS   MAXPODS   REPLICAS   AGE
hello-world-server   StatefulSet/hello-world-server   cpu: 3%/80%   1         10        1          82m
rabbitmq-client      Deployment/rabbitmq-client       cpu: 5%/80%   1         10        1          82m
rabbitmq-worker      Deployment/rabbitmq-worker       cpu: 1%/80%   1         10        1          82m
```

Note the cpu usage increasing.
Perfect! Now increase the load
Note : Change the parameters for stress testing based on need.

```
for ((i=0; i<100; ++i)); do
    seq 1 1000 | xargs -I $ -n1 -P100 curl "http://a9935dc85d16e4c37a458d0279ddc55b-828238954.us-west-2.elb.amazonaws.com:8080/send?msg=test%20utorify"
done
```

[Alternative] Use Apache Benchmarking CLI:

```
ab -n 100000 -c 1000 "http://a9935dc85d16e4c37a458d0279ddc55b-828238954.us-west-2.elb.amazonaws.com:8080/send?msg=test%20utorify"
```

Now open another terminal and keep watching the cpu usage and replica scaling by HPA.

```
k get hpa -w
```

#### Troubleshoot

- If the pod is not running (its state is Pending) and you are deploying to a resource-constrained cluster (eg. local environments like kind or minikube), you may need to adjust CPU and/or memory limits of the cluster. By default, the Operator configures RabbitmqCluster pods to request 1CPU and 2GB of memory.
