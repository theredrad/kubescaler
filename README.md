# Kubescaler
The `kubescaler` is an app to scale k8s cluster node pool to run a bunch of stateful dedicated servers (like dedicated game server using memory state) using kubernetes API inspired by [Mark Mandel blog post](https://www.compoundtheory.com/scaling-dedicated-game-servers-with-kubernetes-part-3-scaling-up-nodes/) . This application is a simple scaler and you can use [Agones](https://agones.dev/site/) for more advanced features.

## Scale Strategy
The scaler follows a simple rule to scale the nodes. There is a buffer size and if the resource request is more than the buffer size, needed new nodes are created and if the resource request decreases, the extra nodes are deleted.

The scaler runs the `scale` method in a loop. If the available resource is smaller than the buffer size, first check the unschedulable nodes and if the resource isn't enough, then resize the pool size to the needed nodes. Otherwise, if the available resource is greater than the buffer size, first calculate the extra nodes and then mark them as unschedulable to prevent scheduling new pods on them. at the end check the unschedulable nodes and deletes expired nodes with no dedicated server pods.

## Permissions
`kubescaler` uses cluster config to manage nodes & pods, so permissions and roles must be applied to the  `kubescaler Deployment`

## Configs
The example config file exists as `config.yaml.exmaple` file. Also, you can set the configs as environment variables in uppercase and snail case format.

* `cloud-provider` : Currently only `digitalocean` is implemented as a cloud provider, but the app supports driver, so you can implement any other provider and only register it (or contribute to this repo and send a pull request). Cloud provider manages the node pool size and deletes extra nodes.
* `cloud-provider-token` : Access token for cloud API
* `cluster-name` : Cluster name
* `node-pool-name` : Node pool name
* `node-selector` : Kubernetes selector to target the dedicated servers nodes. (ex: "role=scalable"). This selector is used to find the nodes that host the dedicated server pods.
* `cluster-kube-config-master-url` : cluster kube config master URL to use k8s API (leave empty if using in-cluster config)
* `cluster-kube-config-path` :  cluster kube config path (leave empty if using in-cluster config)
* `minimum-node-pool-size` : minimum node pool size  
* `maximum-node-pool-size` : maximum node pool size  
* `server-pod-label-name` : dedicated server pod label name  (using to filter the dedicated server pods)
* `server-pod-label-value` : dedicated server pod label value   (using to filter the dedicated server pods)
* `buffer-slot-size` : buffer slot size
* `scale-loop-tick-sec` : scale loop tick duration in seconds 
* `server-cpu-resource-request` : dedicated server pod CPU resource request (in MilliValue)  
* `empty-node-expiration-sec` : empty node expiration duration in seconds (delete node after this time if no pods scheduled)


## TODOs
* [ ] Add kubernetes deployment
* [ ] Add health check
