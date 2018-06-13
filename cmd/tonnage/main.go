package main

import (
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/elementalvoid/tonnage/pkg/log"
	"github.com/elementalvoid/tonnage/pkg/scanner"
)

func main() {
	flag.Parse()
	log.Configure()

	s, err := scanner.NewScanner()
	if err != nil {
		log.Logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Fatal("Unable to connect to cluster.")
	}
	err = s.Scan()
	log.Logger.Infof("Num Nodes: %d", len(s.Nodes))
	// for _, n := range nodeList.Items {

	// 	// clusterResources.accumulate(node)

	// 	if !summaryOnly {
	// 		logger.WithFields(log.Fields{
	// 			"node":    n.Name,
	// 			"numPods": node.NumPods,
	// 			"allocatable": log.Fields{
	// 				"cpu":    node.Allocatable.CPU.String(),
	// 				"memory": node.Allocatable.Memory.String(),
	// 			},
	// 			"requests": log.Fields{
	// 				"cpu":    node.Requests.CPU.String(),
	// 				"memory": node.Requests.Memory.String(),
	// 			},
	// 			"limits": log.Fields{
	// 				"cpu":    node.Limits.CPU.String(),
	// 				"memory": node.Limits.Memory.String(),
	// 			},
	// 		}).Info("Per node usage.")
	// 	}
	// }

	// logger.WithFields(log.Fields{
	// 	"numPods": clusterResources.NumPods,
	// 	"allocatable": log.Fields{
	// 		"cpu":    clusterResources.Allocatable.CPU.String(),
	// 		"memory": clusterResources.Allocatable.Memory.String(),
	// 	},
	// 	"requests": log.Fields{
	// 		"cpu":    clusterResources.Requests.CPU.String(),
	// 		"memory": clusterResources.Requests.Memory.String(),
	// 	},
	// 	"limits": log.Fields{
	// 		"cpu":    clusterResources.Limits.CPU.String(),
	// 		"memory": clusterResources.Limits.Memory.String(),
	// 	},
	// }).Info("Cummulative cluster usage.")
}

// func (r *resourceUsage) accumulate(node resourceUsage) {
// 	r.NumPods += node.NumPods
// 	r.Allocatable.CPU.Add(node.Allocatable.CPU)
// 	r.Allocatable.Memory.Add(node.Allocatable.Memory)
// 	r.Requests.CPU.Add(node.Requests.CPU)
// 	r.Requests.Memory.Add(node.Requests.Memory)
// 	r.Limits.CPU.Add(node.Limits.CPU)
// 	r.Limits.Memory.Add(node.Limits.Memory)
// }
