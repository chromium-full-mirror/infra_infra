// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// backend-scanner is a service to scan caching backends and update a k8s
// configmap which mounted by caching frontend (i.e. the load balancer) and
// backends. It also scales the caching downloader replicas according to the
// count of backend server.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	k8sMetaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sApplyCoreV1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8sApplyMetaV1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sTypedAppsV1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	k8sTypedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

var (
	port                 = flag.Int("port", 8000, "the port the scanner service to listen")
	scanIntervalDuration = flag.Duration("scan-interval", 30*time.Minute, "the interval in seconds to scan all backends")

	namespace = flag.String("namespace", "skylab", "the k8s namespace of caching service")
	appLabel  = flag.String("app-label", "caching-backend", "the app label of the backend pods to scan")
	cmName    = flag.String("configmap-name", "caching-config", "the configmap name to save the generated caching configuration")

	serviceIP   = flag.String("service-ip", "", "the caching service service IP")
	servicePort = flag.Uint("service-port", 8082, "the caching service service port")

	frontendInterface = flag.String("frontend-interface", "bond0", "The NIC name to bring up the frontend service IP")
	frontendLBAlgo    = flag.String("frontend-lb-algo", "rr", "The load balancing/schedulng algorithm of the frontend, see go/peep-fleet-caching-lb-algo for all options")

	cacheSizeInGB     = flag.Uint("cache-size", 750, "The size of cache space in GB.")
	l7Port            = flag.Uint("l7-port", 8083, "the port for caching service L7 balancing")
	otelTraceEndpoint = flag.String("otel-trace-endpoint", "", "The OTel collector endpoint for backend service tracing. e.g. http://127.0.0.1:4317")

	downloaderAppLabel      = flag.String("backend-downloader-app-label", "downloader", "the app label of the downloader which download files from Google Cloud Storage")
	downloaderScalingFactor = flag.Int("backend-downloader-scaling-factor", 4, "the factor to scale the downloader deployment with backends, i.e. '<downlaoder_replica>=<factor> * <backend_count>'")
	downloaderNodeLabel     = flag.String("downloader-node-label", "", "the downloader must run on nodes with the specified label, or they will be evicted")
)

func main() {
	if err := innerMain(); err != nil {
		log.Fatalf("Error: %s", err)
	}
	log.Printf("Done")
}

func innerMain() error {
	flag.Parse()

	s := &backendScanner{
		namespace:   *namespace,
		appLabel:    *appLabel,
		cmName:      *cmName,
		serviceIP:   *serviceIP,
		servicePort: *servicePort,

		backendConf: &nginxConf{
			CacheSizeInGB:     int(*cacheSizeInGB),
			Port:              int(*servicePort),
			L7Port:            int(*l7Port),
			OtelTraceEndpoint: *otelTraceEndpoint,
		},
		frontendConf: &keepalivedConf{
			ServiceIP:   *serviceIP,
			ServicePort: int(*servicePort),
			Interface:   *frontendInterface,
			LBAlgo:      *frontendLBAlgo,
		},
		downloader: &downloader{
			appLabel:    *downloaderAppLabel,
			nodeLabel:   *downloaderNodeLabel,
			scaleFactor: *downloaderScalingFactor,
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/update", s.updateHandler)
	// TODO(guocb): add smarter logic to /startup and /shutdown to update the
	// backend list more efficiently.
	mux.HandleFunc("/startup", s.updateHandler)
	mux.HandleFunc("/shutdown", s.updateHandler)
	svr := http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: mux}
	log.Printf("starting backend scanner")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.scanOnce(ctx); err != nil {
		return err
	}
	log.Printf("Initial configmap created")

	go s.regularlyScan(ctx, *scanIntervalDuration)
	log.Printf("Regular scanning scheduled at every %s", *scanIntervalDuration)

	err := svr.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	return err
}

// backendScanner is for the backend scanner service, which scans the specified
// backends and generate/update specified configMap.
type backendScanner struct {
	namespace   string
	appLabel    string
	cmName      string
	serviceIP   string
	servicePort uint

	backendConf  *nginxConf
	frontendConf *keepalivedConf
	downloader   *downloader
}

func (s *backendScanner) updateHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL)
	if err := s.scanOnce(r.Context()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			log.Printf("Error writing the response body: %s", err)
		}
	}
}

// scanOnce scans the specified caching backends and run the defined actions.
func (s *backendScanner) scanOnce(ctx context.Context) error {
	clientset, err := getK8sClientSet()
	if err != nil {
		return fmt.Errorf("scan once: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	backends, err := getBackends(ctx, clientset.CoreV1().Pods(s.namespace), s.appLabel)
	if err != nil {
		return fmt.Errorf("scan once: %w", err)
	}
	log.Printf("Found backends (app=%s): %+v", s.appLabel, backends)

	s.backendConf.L7Servers = filterRunning(backends)
	nc, err := genConfig("nginx", nginxTemplate, s.backendConf)
	if err != nil {
		return fmt.Errorf("scan once: %w", err)
	}

	s.frontendConf.RealServers = backends
	kc, err := genConfig("keepalived", keepalivedTempalte, s.frontendConf)
	if err != nil {
		return fmt.Errorf("scan once: %w", err)
	}
	d := map[string]string{
		"service-ip":      s.serviceIP,
		"service-port":    fmt.Sprintf("%d", s.servicePort),
		"nginx.conf":      nc,
		"keepalived.conf": kc,
	}
	if err := updateConfigMap(ctx, clientset.CoreV1().ConfigMaps(s.namespace), s.cmName, s.namespace, d); err != nil {
		return fmt.Errorf("scan once: %w", err)
	}

	if err := s.downloader.scale(ctx, clientset.AppsV1().Deployments(*namespace), len(backends)); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if err := s.downloader.evictFromNonCachingNodes(ctx, clientset.CoreV1().Nodes(), clientset.CoreV1().Pods(s.namespace)); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

// filterRunning gets the backends which are running.
func filterRunning(backends []Backend) []Backend {
	var r []Backend
	for _, b := range backends {
		if !b.Terminating {
			r = append(r, b)
		}
	}
	return r
}

// regularlyScan scans at the specified interval duration.
func (s *backendScanner) regularlyScan(ctx context.Context, d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("Regular scanning stopped")
			return
		case <-ticker.C:
			log.Printf("Starting a regular scan")
			if err := s.scanOnce(ctx); err != nil {
				log.Printf("The regular scan failed: %s", err)
			}
		}
	}
}

// updateConfigMap updates the specified k8s configmap with the given data.
func updateConfigMap(ctx context.Context, c k8sTypedCoreV1.ConfigMapInterface, name, namespace string, data map[string]string) error {
	kind := "ConfigMap"
	version := "v1"
	cm := k8sApplyCoreV1.ConfigMapApplyConfiguration{
		TypeMetaApplyConfiguration: k8sApplyMetaV1.TypeMetaApplyConfiguration{
			Kind:       &kind,
			APIVersion: &version,
		},
		ObjectMetaApplyConfiguration: &k8sApplyMetaV1.ObjectMetaApplyConfiguration{
			Name:      &name,
			Namespace: &namespace,
		},
		Data: data,
	}
	if _, err := c.Apply(ctx, &cm, k8sMetaV1.ApplyOptions{FieldManager: "caching-backend-scanner"}); err != nil {
		return fmt.Errorf("update configmap: %w", err)
	}
	return nil
}

// scaleDeployment scales the named deployment to the specified replica.
func scaleDeployment(ctx context.Context, c k8sTypedAppsV1.DeploymentInterface, name string, replica int32) error {
	scale, err := c.GetScale(ctx, name, k8sMetaV1.GetOptions{})
	if err != nil {
		return fmt.Errorf("scale deploy/%s: %w", name, err)
	}
	scale.Spec.Replicas = replica
	_, err = c.UpdateScale(ctx, name, scale, k8sMetaV1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scale deploy/%s: %w", name, err)
	}
	return nil
}

// nginxConf is for rendering the Nginx configuration template.
type nginxConf struct {
	CacheSizeInGB     int       // the caching space size
	Port              int       // the backend port (for L4 balancing)
	L7Servers         []Backend // the L7 servers which cache files based on URI
	L7Port            int       // the port of L7 servers
	OtelTraceEndpoint string    // for OpenTelemetry instruments
}

// keepalivedConf is for rendering the Keepalived configuration template.
type keepalivedConf struct {
	ServiceIP   string    // the service IP (or VIP)
	ServicePort int       // the service port
	RealServers []Backend // the real server (i.e. backend) list
	Interface   string    // the interface Keepalived brings up the service IP
	LBAlgo      string    // the load balancing algorithm to use
}

type Backend struct {
	IP          string // IP of the backend
	Terminating bool   // whether the backend accepts new connections
}

// getBackends gets backend list of selected pods with the specified label.
func getBackends(ctx context.Context, p k8sTypedCoreV1.PodInterface, appLabel string) ([]Backend, error) {
	pods, err := p.List(ctx,
		k8sMetaV1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", appLabel),
			FieldSelector: "status.podIP!=''",
		})
	if err != nil {
		log.Printf("List pods (app=%s): %s", appLabel, err)
		return nil, err
	}
	var backends []Backend
	for _, p := range pods.Items {
		backends = append(
			backends,
			// See below link for why we can use deletion timestamp to check terminating state.
			// https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#terminating
			Backend{IP: p.Status.PodIP, Terminating: (p.DeletionTimestamp != nil)},
		)
	}
	slices.SortFunc(backends, func(a, b Backend) int {
		return strings.Compare(a.IP, b.IP)
	}) // to keep the list stable

	return backends, nil
}

func getK8sClientSet() (*kubernetes.Clientset, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("get K8s client set: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("get K8s client set: %w", err)
	}
	return clientset, nil
}

// genConfig generates the final template data.
func genConfig(templateName, configTmpl string, configData any) (string, error) {
	var buf bytes.Buffer
	tmpl := template.Must(template.New(templateName).Parse(configTmpl))
	if err := tmpl.Execute(&buf, configData); err != nil {
		return "", fmt.Errorf("build config: %w", err)
	}
	return buf.String(), nil
}

type downloader struct {
	appLabel    string // the "app" label of the downloader deployment
	nodeLabel   string // the node label marking on all caching nodes
	scaleFactor int    // the factor of roughly how many downloader we run on each backend node
}

func (d *downloader) scale(ctx context.Context, c k8sTypedAppsV1.DeploymentInterface, backendCount int) error {
	deployments, err := c.List(ctx, k8sMetaV1.ListOptions{LabelSelector: "app=" + d.appLabel})
	if err != nil {
		return fmt.Errorf("scale: %w", err)
	}
	if l := len(deployments.Items); l != 1 {
		return fmt.Errorf("scale: expected 1 but found %d deployment(s) with label app=%s", l, d.appLabel)
	}
	if err := scaleDeployment(ctx, c, deployments.Items[0].Name, d.getReplica(backendCount)); err != nil {
		return fmt.Errorf("scale: %w", err)
	}
	return nil
}

// getReplica returns the downloader replicas according to the given backend
// count. The repicas has a minimum.
func (d *downloader) getReplica(backendCount int) int32 {
	var minimum int32 = 4
	if r := int32(backendCount * d.scaleFactor); r > minimum {
		return r
	}
	return minimum
}

// evictFromNonCachingNodes removes all downloaders running on non-caching
// nodes.
// This may happen when we remove the caching label after we scheduling some
// downloaders on it. If we don't remove the downloaders, their performance may
// be impacted by other workloads scheduled on the same node later.
func (d *downloader) evictFromNonCachingNodes(ctx context.Context, n k8sTypedCoreV1.NodeInterface, p k8sTypedCoreV1.PodInterface) error {
	log.Printf("Downloader runs on nodes with label %q", d.nodeLabel)
	if d.nodeLabel == "" {
		// All nodes are for caching service, so no need to do anything.
		log.Print("Skip the downloader eviction as all nodes are for caching.")
		return nil
	}
	nodes, err := n.List(ctx,
		k8sMetaV1.ListOptions{LabelSelector: d.nodeLabel + "=true"},
	)
	if err != nil {
		return fmt.Errorf("evict downloader from non-caching nodes (%s!=true): %w", d.nodeLabel, err)
	}
	nodeSelector := []string{}
	for _, n := range nodes.Items {
		// We select the non caching nodes, so we use "!=" here.
		nodeSelector = append(nodeSelector, "spec.nodeName!="+n.Name)
	}
	log.Printf("Evicting downloaders of %+v", nodeSelector)
	if err = p.DeleteCollection(ctx, k8sMetaV1.DeleteOptions{},
		k8sMetaV1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", d.appLabel),
			FieldSelector: strings.Join(nodeSelector, ","),
		}); err != nil {
		return fmt.Errorf("evict downloader from non-caching nodes: %w", err)
	}
	return nil
}
