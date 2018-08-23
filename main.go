package main

import (
	"path/filepath"

	flag "github.com/spf13/pflag"

	log "github.com/sirupsen/logrus"

	"github.com/mitchellh/go-homedir"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type resourceUsage struct {
	Allocatable struct {
		CPU    resource.Quantity
		Memory resource.Quantity
	}
	Requests struct {
		CPU    resource.Quantity
		Memory resource.Quantity
	}
	Limits struct {
		CPU    resource.Quantity
		Memory resource.Quantity
	}
	NumPods int
}

var logger = log.New()
var summaryOnly, nodeCountOnly bool
var kubeconfig, kubecontext, nodeLabelSelector, podLabelSelector *string

func init() {
	// handle flags
	if home, err := homedir.Dir(); home != "" && err == nil {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "Absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file")
	}
	kubecontext = flag.String("context", "", "The name of the kubeconfig context to use")
	nodeLabelSelector = flag.String("node-selector", "", "Label selector for nodes to include")
	podLabelSelector = flag.String("pod-selector", "", "Label selector for pods to include")
	debug := flag.BoolP("debug", "d", false, "Enable debug logging")
	colorAlways := flag.BoolP("color-always", "C", false, "Force color output in all cases")
	colorNever := flag.Bool("color-never", false, "Disable color completely")
	json := flag.Bool("json", false, "Enable JSON formatted logging")
	flag.BoolVar(&nodeCountOnly, "node-count", false, "Show only the count of matching nodes")
	flag.BoolVarP(&summaryOnly, "summary", "s", false, "Show only the cummulative cluster summary")
	flag.Parse()

	if *json {
		logger.Formatter = &log.JSONFormatter{}
	} else {
		logger.Formatter = &log.TextFormatter{
			ForceColors:   *colorAlways,
			DisableColors: *colorNever,
			FullTimestamp: true,
		}
	}

	if *debug {
		logger.SetLevel(log.DebugLevel)
	}
}

func main() {
	client, err := getClient()
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("Error creating kube client.")
	}

	nodeList, err := client.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: *nodeLabelSelector,
	})
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("Error listing nodes.")
	}
	logger.WithFields(log.Fields{
		"numNodes":          len(nodeList.Items),
		"nodeLabelselector": *nodeLabelSelector,
	}).Info("Found nodes")
	if nodeCountOnly {
		return
	}

	clusterResources := resourceUsage{}

	for _, n := range nodeList.Items {
		node := resourceUsage{}

		node.Allocatable.CPU = *n.Status.Allocatable.Cpu()
		node.Allocatable.Memory = *n.Status.Allocatable.Memory()

		nodeFieldSelector := fields.OneTermEqualSelector("spec.nodeName", n.GetName()).String()
		podsList, err := client.CoreV1().Pods("").List(metav1.ListOptions{
			// Apply label selector from cli args
			LabelSelector: *podLabelSelector,
			// Limit to Pods on the current node
			FieldSelector: nodeFieldSelector,
		})
		if err != nil {
			logger.WithFields(log.Fields{
				"node":  n.Name,
				"error": err.Error(),
			}).Fatal("Error listing pods for node.")
		}
		node.NumPods = len(podsList.Items)
		for _, p := range podsList.Items {
			for _, c := range p.Spec.Containers {
				reqCPU := c.Resources.Requests["cpu"]
				reqMemory := c.Resources.Requests["memory"]
				limCPU := c.Resources.Limits["cpu"]
				limMemory := c.Resources.Limits["memory"]
				logger.WithFields(log.Fields{
					"pod":       p.Name,
					"container": c.Name,
					"requests": log.Fields{
						"cpu":    reqCPU.String(),
						"memory": reqMemory.String(),
					},
					"limits": log.Fields{
						"cpu":    limCPU.String(),
						"memory": limMemory.String(),
					},
				}).Debug("Per container usage")
				node.Requests.CPU.Add(reqCPU)
				node.Requests.Memory.Add(reqMemory)
				node.Limits.CPU.Add(limCPU)
				node.Limits.Memory.Add(limMemory)
			}
		}

		clusterResources.accumulate(node)

		if !summaryOnly {
			logger.WithFields(log.Fields{
				"node":    n.Name,
				"numPods": node.NumPods,
				"allocatable": log.Fields{
					"cpu":    node.Allocatable.CPU.String(),
					"memory": node.Allocatable.Memory.String(),
				},
				"requests": log.Fields{
					"cpu":    node.Requests.CPU.String(),
					"memory": node.Requests.Memory.String(),
				},
				"limits": log.Fields{
					"cpu":    node.Limits.CPU.String(),
					"memory": node.Limits.Memory.String(),
				},
			}).Info("Per node usage.")
		}
	}

	logger.WithFields(log.Fields{
		"numPods": clusterResources.NumPods,
		"allocatable": log.Fields{
			"cpu":    clusterResources.Allocatable.CPU.String(),
			"memory": clusterResources.Allocatable.Memory.String(),
		},
		"requests": log.Fields{
			"cpu":    clusterResources.Requests.CPU.String(),
			"memory": clusterResources.Requests.Memory.String(),
		},
		"limits": log.Fields{
			"cpu":    clusterResources.Limits.CPU.String(),
			"memory": clusterResources.Limits.Memory.String(),
		},
	}).Info("Cummulative cluster usage.")
}

func (r *resourceUsage) accumulate(node resourceUsage) {
	r.NumPods += node.NumPods
	r.Allocatable.CPU.Add(node.Allocatable.CPU)
	r.Allocatable.Memory.Add(node.Allocatable.Memory)
	r.Requests.CPU.Add(node.Requests.CPU)
	r.Requests.Memory.Add(node.Requests.Memory)
	r.Limits.CPU.Add(node.Limits.CPU)
	r.Limits.Memory.Add(node.Limits.Memory)
}

func getClient() (*kubernetes.Clientset, error) {
	// create a client config with proper kubeconfig and context
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: *kubecontext}).ClientConfig()
	if err != nil {
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	serverInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		logger.WithFields(log.Fields{
			"server": "unknown",
			"err":    err.Error(),
		}).Info("Connected to Kuberenetes API - unable to determine version")
	} else {
		logger.WithFields(log.Fields{
			"server": serverInfo.String(),
		}).Info("Connected to Kuberenetes API")
	}
	return clientset, nil
}
