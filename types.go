package main

import (
	"github.com/dustin/go-humanize"
	"k8s.io/apimachinery/pkg/api/resource"
)

type resources struct {
	CPU    resource.Quantity
	Memory resource.Quantity
}

type containerResources struct {
	Requests resources
	Limits   resources
}

type podResources struct {
	PodName 	  string
	Requests      resources
	Limits        resources
	NumContainers int64
	Containers    []containerResources
}

func (r *podResources) add(container containerResources) {
	r.NumContainers++
	r.Containers = append(r.Containers, container)
	r.Requests.CPU.Add(container.Requests.CPU)
	r.Requests.Memory.Add(container.Requests.Memory)
	r.Limits.CPU.Add(container.Limits.CPU)
	r.Limits.Memory.Add(container.Limits.Memory)
}

func (r *podResources) asTableRow() []string {
	return []string{
		r.PodName,
		"", // Allocatable is not a Pod field
		"", // Allocatable is not a Pod field
		humanize.Comma(r.Requests.CPU.MilliValue()),
		humanize.Comma(r.Requests.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(r.Limits.CPU.MilliValue()),
		humanize.Comma(r.Limits.Memory.ScaledValue(resource.Mega)),
		"", // NumPods is not a Pod field
		humanize.Comma(r.NumContainers),
	}
}

type nodeResources struct {
	NodeName      string
	Allocatable   resources
	Requests      resources
	Limits        resources
	NumPods       int64
	NumContainers int64
	Pods          []podResources
}

func (r *nodeResources) add(pod podResources) {
	r.NumPods++
	r.NumContainers += pod.NumContainers
	r.Pods = append(r.Pods, pod)
	r.Requests.CPU.Add(pod.Requests.CPU)
	r.Requests.Memory.Add(pod.Requests.Memory)
	r.Limits.CPU.Add(pod.Limits.CPU)
	r.Limits.Memory.Add(pod.Limits.Memory)
}

func (r *nodeResources) asTableRow(includePods bool) []string {
	data := []string{r.NodeName}
	if includePods {
		data = append(data, "") // empty Pod Name for table formatting
	}
	data = append(data, []string{
		humanize.Comma(r.Allocatable.CPU.MilliValue()),
		humanize.Comma(r.Allocatable.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(r.Requests.CPU.MilliValue()),
		humanize.Comma(r.Requests.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(r.Limits.CPU.MilliValue()),
		humanize.Comma(r.Limits.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(r.NumPods),
		humanize.Comma(r.NumContainers),
	}...)
	return data
}

type clusterResourceUsage struct {
	Allocatable   resources
	Requests      resources
	Limits        resources
	NumNodes      int64
	NumPods       int64
	NumContainers int64
	Nodes         []nodeResources
}

func (r *clusterResourceUsage) add(node nodeResources) {
	r.NumNodes++
	r.NumPods += node.NumPods
	r.NumContainers += node.NumContainers
	r.Nodes = append(r.Nodes, node)
	r.Allocatable.CPU.Add(node.Allocatable.CPU)
	r.Allocatable.Memory.Add(node.Allocatable.Memory)
	r.Requests.CPU.Add(node.Requests.CPU)
	r.Requests.Memory.Add(node.Requests.Memory)
	r.Limits.CPU.Add(node.Limits.CPU)
	r.Limits.Memory.Add(node.Limits.Memory)
}
