package main

import (
	"errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/theredrad/kubescaler"
	"github.com/theredrad/kubescaler/nodepoolmanager"
	"github.com/theredrad/kubescaler/nodepoolmanager/providers/digitalocean"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"
)

const (
	confCloudProvider       = "cloud-provider"
	confCloudProviderToken  = "cloud-provider-token"
	confClusterName         = "cluster-name"
	confNodePoolName        = "node-pool-name"
	confNodeSelector        = "node-selector"
	confKubeConfigMasterURL = "cluster-kube-config-master-url"
	confKubeConfigPath      = "cluster-kube-config-path"
	confMinNodePoolSize     = "minimum-node-pool-size"
	confMaxNodePoolSize     = "maximum-node-pool-size"
	confPodLabelName        = "server-pod-label-name"
	confPodLabelValue       = "server-pod-label-value"
	confSlotBufferSize      = "buffer-slot-size"
	confScaleLoopTickSec    = "scale-loop-tick-sec"
	confServerCPUResReq     = "server-cpu-resource-request"
	confEmptyNodeExpiration = "empty-node-expiration-sec"
)

func init() {
	initConfig()
}

func main() {
	providerConfig, err := initCloudProviderConfig(viper.GetString(confCloudProvider))
	if err != nil {
		panic(err)
	}
	cloudProvider, err := nodepoolmanager.New(viper.GetString(confCloudProvider), providerConfig)
	if err != nil {
		panic(err)
	}
	k8s, err := kubescaler.NewK8SFromKubeConfig(viper.GetString(confKubeConfigMasterURL), viper.GetString(confKubeConfigPath))
	if err != nil {
		panic(err)
	}

	scaler := kubescaler.NewScaler(cloudProvider, k8s, &kubescaler.Config{
		NodeSelector:        viper.GetString(confNodeSelector),
		MinimumNode:         viper.GetInt(confMinNodePoolSize),
		MaximumNode:         viper.GetInt(confMaxNodePoolSize),
		PodCPURequest:       viper.GetInt64(confServerCPUResReq),
		PodLabelName:        viper.GetString(confPodLabelName),
		PodLabelValue:       viper.GetString(confPodLabelValue),
		EmptyNodeExpiration: time.Duration(viper.GetInt(confEmptyNodeExpiration)) * time.Second,
		BufferSlotSize:      viper.GetInt64(confSlotBufferSize),
	})

	err = scaler.Start()
	if err != nil {
		panic(err)
	}
	defer scaler.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case sig := <-sigs:
		log.Printf("[INFO] got interrupt signal: %s", sig)
	}
	log.Printf("[INFO] exiting")
}

func initConfig() {
	flags := pflag.NewFlagSet(path.Base(os.Args[0]), pflag.ContinueOnError)

	flags.String(confCloudProvider, "digitalocean", "cloud provider name")
	flags.String(confCloudProviderToken, "", "cloud provider token")
	flags.String(confClusterName, "", "cluster name")
	flags.String(confNodePoolName, "", "node pool name")
	flags.String(confKubeConfigMasterURL, "", "kube config master url (leave empty if using in cluster config)")
	flags.String(confKubeConfigPath, "", "kube config path (leave empty if using in cluster config)")
	flags.String(confNodeSelector, "", "node selector label (ex: role=scalable)")
	flags.Int64(confMinNodePoolSize, 2, "minimum node pool size")
	flags.Int64(confMaxNodePoolSize, 3, "maximum node pool size")
	flags.String(confPodLabelName, "", "maximum node pool size")
	flags.String(confPodLabelValue, "", "maximum node pool size")
	flags.Int64(confSlotBufferSize, 4, "buffer slot size")
	flags.Int64(confScaleLoopTickSec, 10, "scale loop tick duration in sec")
	flags.String(confServerCPUResReq, "1m", "server cpu resource request in milli unit")
	flags.Int64(confEmptyNodeExpiration, 120, "empty node expiration time in sec")

	err := flags.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}

	err = viper.BindPFlags(flags)
	if err != nil {
		panic(err)
	}

	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	err = viper.ReadInConfig()
	if err != nil {
		log.Printf("[WARN] error while reading config file: %s", err)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func initCloudProviderConfig(driver string) (interface{}, error) {
	switch driver {
	case digitalocean.DriverName:
		return &digitalocean.Config{
			Token:        viper.GetString(confCloudProviderToken),
			ClusterName:  viper.GetString(confClusterName),
			NodePoolName: viper.GetString(confNodePoolName),
		}, nil
	default:
		return nil, errors.New("invalid cloud provider driver")
	}
}
