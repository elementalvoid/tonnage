package scanner

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// NodeList is a list of Nodes.
type NodeList []Node

// Node is a minimilistic representation of a Kubernetes Node type.
type Node struct {
	Name string
	// Cloud struct {
	// 	Provider     string
	// 	InstanceType string
	// }
	Labels    map[string]string
	NodeInfo  v1.NodeSystemInfo
	Resources struct {
		Allocatable v1.ResourceList
		// Allocatable struct {
		// 	CPU    resource.Quantity
		// 	Memory resource.Quantity
		// }
		Allocated v1.ResourceRequirements
	}
	Pods PodList
}

// NewNode returns an initialized Node.
func NewNode() Node {
	n := Node{}
	n.Resources.Allocatable = make(map[v1.ResourceName]resource.Quantity, 2)
	n.Resources.Allocated.Requests = make(map[v1.ResourceName]resource.Quantity, 2)
	n.Resources.Allocated.Limits = make(map[v1.ResourceName]resource.Quantity, 2)
	return n
}

// PodList is a list of Pods.
type PodList []Pod

// Pod is a minimilistic representation of a Kubernetes Pod type.
type Pod struct {
	Name       string
	Namespace  string
	Labels     map[string]string
	Containers []Container
}

// Container is a minimilistic representation of a Kubernetes Containter type.
type Container struct {
	Name      string
	Image     string
	Resources v1.ResourceRequirements
}
