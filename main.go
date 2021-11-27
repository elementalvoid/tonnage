package main

import (
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"os"
	"sync"

	flag "github.com/spf13/pflag"

	log "github.com/sirupsen/logrus"

	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var logger = log.New()
var summaryOnly, includePods bool
var kubeconfig, kubecontext, nodeLabelSelector, podLabelSelector *string

var clusterStats = clusterResourceUsage{}

func init() {
	// handle flags
	debug := flag.BoolP("debug", "d", false, "Enable debug logging")
	//colorAlways := flag.BoolP("color-always", "C", false, "Force color output in all cases")
	//colorNever := flag.Bool("color-never", false, "Disable color completely")
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to a kubeconfig file. Uses ~/.kube/config by default.")
	kubecontext = flag.String("context", "", "The name of the kubeconfig context to use")
	nodeLabelSelector = flag.StringP("node-selector", "N", "", "Label selector for nodes to include")
	podLabelSelector = flag.StringP("pod-selector", "P", "", "Label selector for pods to include")
	flag.BoolVarP(&includePods, "include-pods", "p", false, "Include Pod resource data in output")
	flag.BoolVarP(&summaryOnly, "summary", "s", false, "Show only the cumulative cluster summary")
	help := flag.BoolP("help", "h", false, "Show help")
	flag.Parse()

	if *help {
		println("Usage: tonnage [options]")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if summaryOnly && includePods {
		flag.PrintDefaults()
		log.Fatal("Invalid flags: --summary (-s) and --include-pods (-p) are mutually exclusive. Choose only one.")
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

	ctx := context.Background()
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: *nodeLabelSelector,
	})
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("Error listing nodes.")
	}
	logger.WithFields(log.Fields{
		"numNodes":          len(nodeList.Items),
		"nodeLabelSelector": *nodeLabelSelector,
		"podLabelSelector":  *podLabelSelector,
	}).Info("Found nodes")

	wg := sync.WaitGroup{}
	wg.Add(len(nodeList.Items))

	progress := mpb.New(mpb.WithWaitGroup(&wg), mpb.WithWidth(64))

	barName := "Nodes: "
	bar := progress.AddBar(
		int64(len(nodeList.Items)),
		mpb.PrependDecorators(decor.Name(barName), decor.CountersNoUnit("%d / %d", decor.WCSyncWidth)),
		mpb.AppendDecorators(decor.Percentage()),
	)

	for _, n := range nodeList.Items {
		go func(node v1.Node) {
			defer wg.Done()
			processNode(client, node, progress)
			bar.Increment()
		}(n)
	}
	if len(nodeList.Items) == 0 {
		// must proceed the progress bar to 100%
		bar.SetTotal(0, true)
	}
	progress.Wait()
	printTable(clusterStats)
}

func printTable(clusterStats clusterResourceUsage) {
	table := tablewriter.NewWriter(os.Stdout)
	var header []string
	if includePods {
		header = []string{
			"Node Name",
			"Pod Name",
			"Allocatable CPU (milli)",
			"Allocatable Memory (Mi)",
			"Requested CPU (milli)",
			"Requested Memory (Mi)",
			"Limits CPU (milli)",
			"Limits Memory (Mi)",
			"# Pods",
			"# Containers",
		}
	} else {
		header = []string{
			"Node Name",
			"Allocatable CPU (milli)",
			"Allocatable Memory (Mi)",
			"Requested CPU (milli)",
			"Requested Memory (Mi)",
			"Limits CPU (milli)",
			"Limits Memory (Mi)",
			"# Pods",
			"# Containers",
		}
	}
	table.SetHeader(header)
	table.SetAutoFormatHeaders(false)

	var data [][]string
	if !summaryOnly {
		for _, n := range clusterStats.Nodes {
			data = append(data, n.asTableRow(includePods))
			if includePods {
				table.SetRowLine(true)
				table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
				for _, p := range n.Pods {
					var pData []string
					pData = append(pData, n.NodeName)
					pData = append(pData, p.asTableRow()...)
					data = append(data, pData)
				}
			}
		}
		table.AppendBulk(data)
	}

	footer := []string{"Cluster Total"}
	if includePods {
		footer = append(footer, "") // empty Pod Name for table formatting
	}
	footer = append(footer, []string{
		humanize.Comma(clusterStats.Allocatable.CPU.MilliValue()),
		humanize.Comma(clusterStats.Allocatable.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(clusterStats.Requests.CPU.MilliValue()),
		humanize.Comma(clusterStats.Requests.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(clusterStats.Limits.CPU.MilliValue()),
		humanize.Comma(clusterStats.Limits.Memory.ScaledValue(resource.Mega)),
		humanize.Comma(clusterStats.NumPods),
		humanize.Comma(clusterStats.NumContainers),
	}...)

	table.SetFooter(footer)
	table.Render()
}

func processNode(client *kubernetes.Clientset, node v1.Node, progress *mpb.Progress) {
	nodeStats := nodeResources{}
	nodeStats.NodeName = node.Name

	nodeStats.Allocatable.CPU = *node.Status.Allocatable.Cpu()
	nodeStats.Allocatable.Memory = *node.Status.Allocatable.Memory()

	nodeFieldSelector := fields.OneTermEqualSelector("spec.nodeName", node.GetName()).String()
	ctx := context.Background()
	podsList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		// Apply label selector from cli args
		LabelSelector: *podLabelSelector,
		// Limit to Pods on the current Node
		FieldSelector: nodeFieldSelector,
	})
	if err != nil {
		logger.WithFields(log.Fields{
			"node":  node.Name,
			"error": err.Error(),
		}).Fatal("Error listing pods for Node.")
	}

	barName := fmt.Sprintf("Pods/Containers for %s: ", node.GetName())
	bar := progress.AddBar(
		int64(len(podsList.Items)),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(decor.Name(barName), decor.CountersNoUnit("%d / %d", decor.WCSyncWidth)),
		mpb.AppendDecorators(decor.Percentage()),
	)

	for _, p := range podsList.Items {
		if p.Status.Phase != "Running" {
			bar.Increment()
			continue
		}
		pr := podResources{
			PodName:    p.Name,
			Requests:   resources{},
			Limits:     resources{},
			Containers: nil,
		}
		for _, c := range p.Spec.Containers {
			cr := containerResources{
				Requests: resources{
					CPU:    c.Resources.Requests["cpu"],
					Memory: c.Resources.Requests["memory"],
				},
				Limits: resources{
					CPU:    c.Resources.Limits["cpu"],
					Memory: c.Resources.Limits["memory"],
				},
			}
			pr.add(cr)

			bar.Increment()
		}
		nodeStats.add(pr)
	}
	if len(podsList.Items) == 0 {
		// must proceed the progress bar to 100%
		bar.SetTotal(0, true)
	}
	clusterStats.add(nodeStats)
}

func getClient() (*kubernetes.Clientset, error) {
	// create a client config with proper kubeconfig and context
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if *kubeconfig != "" {
		loadingRules.ExplicitPath = *kubeconfig
	}
	if *kubecontext != "" {
		configOverrides.CurrentContext = *kubecontext
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, configOverrides).ClientConfig()
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
		}).Info("Connected to Kubernetes API - unable to determine version")
	} else {
		logger.WithFields(log.Fields{
			"server": serverInfo.String(),
		}).Info("Connected to Kubernetes API")
	}
	return clientset, nil
}
