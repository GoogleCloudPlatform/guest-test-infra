//go:build cit
// +build cit

package loadbalancer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/protobuf/proto"
)

var client = http.Client{Timeout: 5 * time.Second}

// These functions only exist to make test result names less ambiguous
func TestL3Backend(t *testing.T) { runBackend(t) }
func TestL7Backend(t *testing.T) { runBackend(t) }

func setupFirewall(t *testing.T) {
	if utils.IsWindows() {
		out, err := utils.RunPowershellCmd(`New-NetFirewallRule -DisplayName 'open8080inbound' -LocalPort 8080 -Action Allow -Profile 'Public' -Protocol TCP -Direction Inbound`)
		if err != nil {
			t.Fatalf("could not allow inbound traffic on port 8080: %s %s %v", out.Stdout, out.Stderr, err)
		}
		out, err = utils.RunPowershellCmd(`New-NetFirewallRule -DisplayName 'open8080outbound' -LocalPort 8080 -Action Allow -Profile 'Public' -Protocol TCP -Direction Outbound`)
		if err != nil {
			t.Fatalf("could not allow outbound traffic on port 8080: %s %s %v", out.Stdout, out.Stderr, err)
		}
	}
}

func runBackend(t *testing.T) {
	ctx := utils.Context(t)
	setupFirewall(t)
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("could not get hostname: %v", err)
	}
	var mu sync.RWMutex
	srv := http.Server{
		Addr: ":8080",
	}
	var count int
	c := make(chan struct{})
	stop := make(chan struct{})
	go func() {
	countloop:
		for {
			select {
			case <-c:
				count++
			case <-stop:
				break countloop
			}
		}
		mu.Lock()
		defer mu.Unlock()
		srv.Shutdown(ctx)
	}()
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		if strings.Contains(req.Host, l3IlbIP4Addr) || strings.Contains(req.Host, l7IlbIP4Addr) {
			c <- struct{}{}
		}
		body, err := io.ReadAll(req.Body)
		io.WriteString(w, host)
		w.WriteHeader(http.StatusOK)
		if err == nil && string(body) == "stop" {
			stop <- struct{}{}
		}
	})
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		t.Errorf("Failed to serve http: %v", err)
	}
	if count < 1 {
		t.Errorf("Receieved zero requests through load balancer")
	}
}

func getTargetWithTimeout(ctx context.Context, t *testing.T, target string, body string) (string, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/", target), strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create http request to %s: %v", target, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err == io.EOF {
		err = nil
	}
	return string(respBody), err
}

func checkBackendsInLoadBalancer(ctx context.Context, t *testing.T, lbip string) {
	var resp1, resp2 string
	for ctx.Err() == nil {
		time.Sleep(3 * time.Second) // Wait enough time for the health check and load balancer to update
		r, err := getTargetWithTimeout(ctx, t, lbip, "stop")
		if err != nil || r == "no healthy upstream" {
			continue
		}
		if resp1 == "" {
			resp1 = r
			continue
		}
		resp2 = r
		break
	}
	if err := ctx.Err(); err != nil {
		t.Errorf("context expired before test completed: %v", err)
	}
	if resp1 == resp2 {
		t.Errorf("wanted different responses from both http requests, got %s for both", resp1)
	}
}

func waitForBackends(ctx context.Context, t *testing.T, backend1 string, backend2 string) {
	t.Cleanup(func() {
		if t.Failed() {
			// Stop backends on failure (in case we didn't make it that far
			getTargetWithTimeout(ctx, t, backend1, "stop")
			getTargetWithTimeout(ctx, t, backend2, "stop")
		}
	})
	wait := func(backend string) {
		for {
			if err := ctx.Err(); err != nil {
				t.Fatalf("test context expired before %s is serving: %v", backend, err)
			}
			_, err := getTargetWithTimeout(ctx, t, backend, "")
			if err == nil {
				break
			}
		}
	}
	wait(backend1)
	wait(backend2)
}

func setupLoadBalancer(ctx context.Context, t *testing.T, lbType, backend1, backend2, lbip string) {
	waitFor := func(op *compute.Operation, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := op.Wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	zone, err := utils.GetMetadata(ctx, "instance", "zone")
	if err != nil {
		t.Fatalf("could not get zone: %v", err)
	}
	zone = path.Base(zone)
	project, err := utils.GetMetadata(ctx, "project", "project-id")
	if err != nil {
		t.Fatalf("could not get project: %v", err)
	}
	backend1, err = utils.GetRealVMName(backend1)
	if err != nil {
		t.Fatalf("could not get backend1 name: %v", err)
	}
	backend1, _, _ = strings.Cut(backend1, ".")
	backend2, err = utils.GetRealVMName(backend2)
	if err != nil {
		t.Fatalf("could not get backend2 name: %v", err)
	}
	backend2, _, _ = strings.Cut(backend2, ".")
	// TODO: implement all necessary steps in daisy to do this inside the test framework
	negClient, err := compute.NewNetworkEndpointGroupsRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	backendServiceClient, err := compute.NewRegionBackendServicesRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	healthCheckClient, err := compute.NewRegionHealthChecksRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	urlMapsClient, err := compute.NewRegionUrlMapsRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	httpProxyClient, err := compute.NewRegionTargetHttpProxiesRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	networkClient, err := compute.NewNetworksRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	forwardingRuleClient, err := compute.NewForwardingRulesRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	zoneClient, err := compute.NewZonesRESTClient(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	zoneGetReq := &computepb.GetZoneRequest{
		Project: project,
		Zone:    zone,
	}
	zoneproto, err := zoneClient.Get(ctx, zoneGetReq)
	if err != nil {
		t.Fatalf("%v", err)
	}
	region := path.Base(*zoneproto.Region)
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("%v", err)
	}
	network, err := utils.GetMetadata(ctx, "instance", "network-interfaces", "0", "network")
	if err != nil {
		t.Fatalf("%v", err)
	}
	network = path.Base(network)
	networkGetReq := &computepb.GetNetworkRequest{
		Project: project,
		Network: network,
	}
	networkproto, err := networkClient.Get(ctx, networkGetReq)
	if err != nil {
		t.Fatalf("%v", err)
	}
	var subnetwork string
	for _, subnetName := range networkproto.Subnetworks {
		if !strings.Contains(subnetName, "proxy") {
			subnetwork = subnetName
		}
	}
	network = *networkproto.SelfLink
	hostname, _, _ = strings.Cut(hostname, ".")
	negName := hostname + "-neg"
	healthCheckName := hostname + "-httphealthcheck"
	backendName := hostname + "-backend"
	urlMapName := hostname + "-urlmap"
	httpProxyName := hostname + "-httpproxy"
	forwardingRuleName := hostname + "-forwardingrule"

	t.Cleanup(func() {
		ctx := context.TODO() // we want to fire off attempts to clean up even if the test context has expired
		tryWait := func(op *compute.Operation, err error) {
			if err == nil {
				op.Wait(ctx)
			}
		}
		deleteFRReq := &computepb.DeleteForwardingRuleRequest{
			Project:        project,
			Region:         region,
			ForwardingRule: forwardingRuleName,
		}
		tryWait(forwardingRuleClient.Delete(ctx, deleteFRReq))
		if lbType == "L7" { // Clean up extra resources from L7 load balancers
			deleteHttpProxyReq := &computepb.DeleteRegionTargetHttpProxyRequest{
				Project:         project,
				Region:          region,
				TargetHttpProxy: httpProxyName,
			}
			tryWait(httpProxyClient.Delete(ctx, deleteHttpProxyReq))
			deleteUrlMapReq := &computepb.DeleteRegionUrlMapRequest{
				Project: project,
				Region:  region,
				UrlMap:  urlMapName,
			}
			tryWait(urlMapsClient.Delete(ctx, deleteUrlMapReq))
		}
		deleteBEReq := &computepb.DeleteRegionBackendServiceRequest{ // Delete backend
			Project:        project,
			Region:         region,
			BackendService: backendName,
		}
		tryWait(backendServiceClient.Delete(ctx, deleteBEReq))
		// Delete health check
		deleteHcReq := &computepb.DeleteRegionHealthCheckRequest{
			Project:     project,
			Region:      region,
			HealthCheck: healthCheckName,
		}
		healthCheckClient.Delete(ctx, deleteHcReq)
		deleteNegReq := &computepb.DeleteNetworkEndpointGroupRequest{ // delete NEG
			Project:              project,
			Zone:                 zone,
			NetworkEndpointGroup: negName,
		}
		negClient.Delete(ctx, deleteNegReq)

		negClient.Close()
		healthCheckClient.Close()
		urlMapsClient.Close()
		networkClient.Close()
		zoneClient.Close()
		httpProxyClient.Close()
		backendServiceClient.Close()
		forwardingRuleClient.Close()
	})

	switch lbType {
	case "L3":
		// Create network endpoint group in lbnet and lbsubnet with GCE_VM_IP type
		neg := &computepb.NetworkEndpointGroup{
			Name:                &negName,
			NetworkEndpointType: proto.String("GCE_VM_IP"),
			Network:             &network,
			Subnetwork:          &subnetwork,
		}
		insertNegReq := &computepb.InsertNetworkEndpointGroupRequest{
			Project:                      project,
			Zone:                         zone,
			NetworkEndpointGroupResource: neg,
		}
		waitFor(negClient.Insert(ctx, insertNegReq))
	case "L7":
		// Create network endpoint group in lbnet and lbsubnet with GCE_VM_IP_PORT type
		neg := &computepb.NetworkEndpointGroup{
			Name:                &negName,
			NetworkEndpointType: proto.String("GCE_VM_IP_PORT"),
			Network:             &network,
			Subnetwork:          &subnetwork,
			DefaultPort:         proto.Int32(8080),
		}
		insertNegReq := &computepb.InsertNetworkEndpointGroupRequest{
			Project:                      project,
			Zone:                         zone,
			NetworkEndpointGroupResource: neg,
		}
		waitFor(negClient.Insert(ctx, insertNegReq))
	}

	// Add instance endpoints of backend1 and backend2 to NEG
	backendsRes := &computepb.NetworkEndpointGroupsAttachEndpointsRequest{
		NetworkEndpoints: []*computepb.NetworkEndpoint{
			{Instance: proto.String(fmt.Sprintf("projects/%s/zones/%s/instances/%s", project, zone, backend1))},
			{Instance: proto.String(fmt.Sprintf("projects/%s/zones/%s/instances/%s", project, zone, backend2))},
		},
	}
	addBackendsReq := &computepb.AttachNetworkEndpointsNetworkEndpointGroupRequest{
		NetworkEndpointGroup: negName,
		NetworkEndpointGroupsAttachEndpointsRequestResource: backendsRes,
		Project: project,
		Zone:    zone,
	}
	waitFor(negClient.AttachNetworkEndpoints(ctx, addBackendsReq))
	// Create http health on port 8080
	httpHc := &computepb.HTTPHealthCheck{
		PortSpecification: proto.String("USE_FIXED_PORT"),
		Port:              proto.Int32(8080),
	}
	hcRes := &computepb.HealthCheck{
		CheckIntervalSec: proto.Int32(1),
		TimeoutSec:       proto.Int32(1),
		Name:             &healthCheckName,
		HttpHealthCheck:  httpHc,
		Type:             proto.String("HTTP"),
	}
	insertHealthCheckReq := &computepb.InsertRegionHealthCheckRequest{
		Project:             project,
		HealthCheckResource: hcRes,
		Region:              region,
	}
	waitFor(healthCheckClient.Insert(ctx, insertHealthCheckReq))

	switch lbType {
	case "L3":
		// Create INTERNAL tcp backend service with health check
		backendService := &computepb.BackendService{
			HealthChecks: []string{fmt.Sprintf("projects/%s/regions/%s/healthChecks/%s", project, region, healthCheckName)},
			Backends: []*computepb.Backend{
				{Group: proto.String(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/networkEndpointGroups/%s", project, zone, negName))},
			},
			Name:                &backendName,
			LoadBalancingScheme: proto.String("INTERNAL"),
			Protocol:            proto.String("TCP"),
		}
		backendInsertReq := &computepb.InsertRegionBackendServiceRequest{
			Project:                project,
			BackendServiceResource: backendService,
			Region:                 region,
		}
		waitFor(backendServiceClient.Insert(ctx, backendInsertReq))

		// Create forwarding rule to send traffic to to the load balancer
		forwardingRule := &computepb.ForwardingRule{
			LoadBalancingScheme: proto.String("INTERNAL"),
			Network:             &network,
			Subnetwork:          &subnetwork,
			BackendService:      proto.String(fmt.Sprintf("projects/%s/regions/%s/backendServices/%s", project, region, backendName)),
			IPAddress:           &lbip,
			IPProtocol:          proto.String("TCP"),
			Ports:               []string{"8080"},
			Name:                &forwardingRuleName,
		}
		forwardingRuleReq := &computepb.InsertForwardingRuleRequest{
			Project:                project,
			Region:                 region,
			ForwardingRuleResource: forwardingRule,
		}
		waitFor(forwardingRuleClient.Insert(ctx, forwardingRuleReq))
	case "L7":
		// Create INTERNAL_MANAGED http backend service with health check
		backendService := &computepb.BackendService{
			HealthChecks: []string{fmt.Sprintf("projects/%s/regions/%s/healthChecks/%s", project, region, healthCheckName)},
			Backends: []*computepb.Backend{
				{
					Group:              proto.String(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/networkEndpointGroups/%s", project, zone, negName)),
					BalancingMode:      proto.String("RATE"),
					MaxRatePerEndpoint: proto.Float32(512),
				},
			},
			Name:                &backendName,
			LoadBalancingScheme: proto.String("INTERNAL_MANAGED"),
			Protocol:            proto.String("HTTP"),
		}
		backendInsertReq := &computepb.InsertRegionBackendServiceRequest{
			Project:                project,
			BackendServiceResource: backendService,
			Region:                 region,
		}
		waitFor(backendServiceClient.Insert(ctx, backendInsertReq))

		// Create URL map to route requests to the backend
		insertUrlMapReq := &computepb.InsertRegionUrlMapRequest{
			Project: project,
			Region:  region,
			UrlMapResource: &computepb.UrlMap{
				Name:           &urlMapName,
				DefaultService: proto.String(fmt.Sprintf("projects/%s/regions/%s/backendServices/%s", project, region, backendName)),
			},
		}
		waitFor(urlMapsClient.Insert(ctx, insertUrlMapReq))

		// Create http proxy to route requests to the url map
		proxyInsertReq := &computepb.InsertRegionTargetHttpProxyRequest{
			Project: project,
			Region:  region,
			TargetHttpProxyResource: &computepb.TargetHttpProxy{
				Name:   &httpProxyName,
				UrlMap: proto.String(fmt.Sprintf("projects/%s/regions/%s/urlMaps/%s", project, region, urlMapName)),
			},
		}
		waitFor(httpProxyClient.Insert(ctx, proxyInsertReq))

		// Create forwarding rule to send traffic to to the proxy
		forwardingRule := &computepb.ForwardingRule{
			LoadBalancingScheme: proto.String("INTERNAL_MANAGED"),
			Network:             &network,
			Subnetwork:          &subnetwork,
			Target:              proto.String(fmt.Sprintf("projects/%s/regions/%s/targetHttpProxies/%s", project, region, httpProxyName)),
			IPAddress:           &lbip,
			IPProtocol:          proto.String("TCP"),
			PortRange:           proto.String("8080"),
			Name:                &forwardingRuleName,
		}
		forwardingRuleReq := &computepb.InsertForwardingRuleRequest{
			Project:                project,
			Region:                 region,
			ForwardingRuleResource: forwardingRule,
		}
		waitFor(forwardingRuleClient.Insert(ctx, forwardingRuleReq))
	}
}

func TestL3Client(t *testing.T) {
	ctx := utils.Context(t)
	setupFirewall(t)
	waitForBackends(ctx, t, l3backendVM1IP4addr+":8080", l3backendVM2IP4addr+":8080")
	setupLoadBalancer(ctx, t, "L3", "l3backend1", "l3backend2", l3IlbIP4Addr)
	checkBackendsInLoadBalancer(ctx, t, l3IlbIP4Addr+":8080")
}

func TestL7Client(t *testing.T) {
	ctx := utils.Context(t)
	setupFirewall(t)
	waitForBackends(ctx, t, l7backendVM1IP4addr+":8080", l7backendVM2IP4addr+":8080")
	setupLoadBalancer(ctx, t, "L7", "l7backend1", "l7backend2", l7IlbIP4Addr)
	checkBackendsInLoadBalancer(ctx, t, l7IlbIP4Addr+":8080")
}
