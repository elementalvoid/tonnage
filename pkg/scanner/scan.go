package scanner

import (
	"fmt"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/elementalvoid/tonnage/pkg/log"
)

var kubeConfig, kubeContext, nodeLabelSelector, podLabelSelector, namespace *string

func init() {
	if home, err := homedir.Dir(); home != "" && err == nil {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "Absolute path to the kubeconfig file")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file")
	}
	kubeContext = flag.String("context", "", "The name of the kubeconfig context to use")
	nodeLabelSelector = flag.String("node-selector", "", "Label selector for nodes to include")
	podLabelSelector = flag.String("pod-selector", "", "Label selector for pods to include")
	namespace = flag.StringP("namespace", "n", "namespace", "Limit Pod selection to a namespace.")
}

// Scanner scans a Kubernetes cluster gathering Node and Pod information.
type Scanner struct {
	Client *kubernetes.Clientset
	filter struct {
		nodeSelector *string
		podSelector  *string
		namespace    *string
	}
	Nodes NodeList
}

// NewScanner creates a new scanner with an initialized k8s client.
func NewScanner() (Scanner, error) {
	s := Scanner{}
	c, err := getClient()
	if err != nil {
		return s, err
	}
	s.Client = c
	s.filter.nodeSelector = nodeLabelSelector
	s.filter.podSelector = podLabelSelector
	s.filter.namespace = namespace
	return s, nil
}

func getClient() (*kubernetes.Clientset, error) {
	// create a client config with proper kubeconfig and context
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeConfig},
		&clientcmd.ConfigOverrides{CurrentContext: *kubeContext}).ClientConfig()
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
		log.Logger.WithFields(logrus.Fields{
			"server": "unknown",
			"err":    err.Error(),
		}).Info("Connected to Kuberenetes API - unable to determine version")
	} else {
		log.Logger.WithFields(logrus.Fields{
			"server": serverInfo.String(),
		}).Info("Connected to Kuberenetes API")
	}
	return clientset, nil
}

func (s *Scanner) scanNodes() error {
	nodeList, err := s.Client.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: *nodeLabelSelector,
	})
	if err != nil {
		return fmt.Errorf("Unable to list cluster Nodes: %s", err.Error())
	}
	log.Logger.WithFields(logrus.Fields{
		"numNodes":          len(nodeList.Items),
		"nodeLabelselector": *nodeLabelSelector,
	}).Info("Found nodes.")
	s.Nodes = NodeList{}
	for _, n := range nodeList.Items {
		log.Logger.WithFields(logrus.Fields{
			"name": n.Name,
		}).Debug("Processing node.")
		node := NewNode()
		node.Name = n.Name
		node.Labels = n.Labels
		node.Resources.Allocatable["cpu"] = *n.Status.Allocatable.Cpu()
		node.Resources.Allocatable["memory"] = *n.Status.Allocatable.Memory()
		node.NodeInfo = n.Status.NodeInfo
		s.Nodes = append(s.Nodes, node)
	}
	return nil
}

func (s *Scanner) scanPods() error {
	for _, node := range s.Nodes {
		nodeFieldSelector := fields.OneTermEqualSelector("spec.nodeName", node.Name).String()
		// TODO: Filter by namespace
		podList, err := s.Client.CoreV1().Pods("").List(metav1.ListOptions{
			// Apply label selector from cli args
			LabelSelector: *podLabelSelector,
			// Limit to Pods on the current node
			FieldSelector: nodeFieldSelector,
		})
		log.Logger.WithFields(logrus.Fields{
			"nodeName":         node.Name,
			"numPods":          len(podList.Items),
			"podLabelSelector": *podLabelSelector,
		}).Debug("Found pods for node.")
		if err != nil {
			log.Logger.WithFields(logrus.Fields{
				"node":  node.Name,
				"error": err.Error(),
			}).Fatal("Error listing pods for node.")
		}
		for _, p := range podList.Items {
			log.Logger.WithFields(logrus.Fields{
				"name":          p.Name,
				"numContainers": len(p.Spec.Containers),
			}).Debug("Processing pod.")
			for _, c := range p.Spec.Containers {
				reqCPU := c.Resources.Requests.Cpu()
				reqMemory := c.Resources.Requests.Memory()
				limCPU := c.Resources.Limits.Cpu()
				limMemory := c.Resources.Limits.Memory()
				log.Logger.WithFields(logrus.Fields{
					"pod":       p.Name,
					"container": c.Name,
					"requests": logrus.Fields{
						"cpu":    reqCPU.String(),
						"memory": reqMemory.String(),
					},
					"limits": logrus.Fields{
						"cpu":    limCPU.String(),
						"memory": limMemory.String(),
					},
				}).Debug("Per container usage")
				node.Resources.Allocated.Requests.Cpu().Add(*reqCPU)
				node.Resources.Allocated.Requests.Memory().Add(*reqMemory)
				node.Resources.Allocated.Limits.Cpu().Add(*limCPU)
				node.Resources.Allocated.Limits.Memory().Add(*limMemory)
			}
		}
	}
	return nil
}

// Scan connects to the cluster and enumerates the Nodes and Pods matching
// the configured filters (Node and Pod selectors and Namespace).
func (s *Scanner) Scan() error {
	// TODO: Usage channels for procseeing each item since JSON unmarshalling is sloooowwww.

	// clusterResources := resourceUsage{}
	err := s.scanNodes()
	if err != nil {
		return err
	}
	err = s.scanPods()
	if err != nil {
		return err
	}
	return nil
}
