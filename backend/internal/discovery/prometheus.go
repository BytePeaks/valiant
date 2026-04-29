package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"valiant/internal/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ErrNoPrometheusFound is returned when no valid Prometheus endpoint can be discovered.
var ErrNoPrometheusFound = errors.New("prometheus auto-discovery: no valid endpoint found")

const (
	maxCandidates     = 5
	shortCircuitScore = 7
)

type candidate struct {
	namespace   string
	name        string
	clusterIP   string
	port        int32
	portName    string
	url         string
	score       int
	isFallback  bool
	labels      map[string]string
	hasSelector bool
}

// PrometheusDiscoverer discovers a Prometheus-compatible endpoint from Kubernetes Services.
type PrometheusDiscoverer struct {
	k8sClient  kubernetes.Interface
	httpClient *http.Client
	cfg        config.Config
}

// NewPrometheusDiscoverer creates a discoverer using the same K8s config pattern as the collector.
func NewPrometheusDiscoverer(cfg config.Config) (*PrometheusDiscoverer, error) {
	var restCfg *rest.Config
	var err error
	if cfg.Kubernetes.KubeConfigPath != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubernetes.KubeConfigPath)
	} else {
		restCfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}
	k8sClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
		},
	}
	return &PrometheusDiscoverer{k8sClient: k8sClient, httpClient: httpClient, cfg: cfg}, nil
}

// preFilter removes headless, uninitialized, and ExternalName services.
// Optional best-effort: also discard if net.ParseIP(clusterIP).IsGlobalUnicast() == false
func preFilter(svcs []corev1.Service) []corev1.Service {
	out := svcs[:0]
	for _, s := range svcs {
		if s.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		if s.Spec.ClusterIP == "" || s.Spec.ClusterIP == "None" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// knownPortNames are port names that strongly suggest a Prometheus HTTP endpoint.
var knownPortNames = map[string]bool{
	"http":       true,
	"web":        true,
	"http-web":   true,
	"prometheus": true,
}

// selectPort picks the best port from a Service's port list.
// Returns (portNumber, portName, accepted, isFallback).
func selectPort(ports []corev1.ServicePort) (int32, string, bool, bool) {
	if len(ports) == 0 {
		return 0, "", false, false
	}
	// Rule 1: port 9090 wins
	for _, p := range ports {
		if p.Port == 9090 {
			return p.Port, p.Name, true, false
		}
	}
	// Rule 2: known port name
	for _, p := range ports {
		if knownPortNames[p.Name] {
			return p.Port, p.Name, true, false
		}
	}
	// Rule 3: single-port fallback (score penalty applied in scoring step)
	if len(ports) == 1 {
		return ports[0].Port, ports[0].Name, true, true
	}
	// Rule 4: multiple ports, none matching — reject
	return 0, "", false, false
}

// buildURL builds the candidate URL. Port 9091 prefers https.
func buildURL(clusterIP string, port int32) string {
	scheme := "http"
	if port == 9091 {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, clusterIP, port)
}

func scoreCandidate(c candidate) int {
	s := 0
	if c.port == 9090 {
		s += 2
	}
	if knownPortNames[c.portName] {
		s += 2
	}
	if strings.Contains(strings.ToLower(c.name), "prometheus") {
		s += 2
	}
	if strings.Contains(strings.ToLower(c.namespace), "monitoring") {
		s += 2
	}
	if c.labels["app.kubernetes.io/component"] == "server" {
		s += 1
	}
	if strings.Contains(strings.ToLower(c.name), "thanos") {
		s += 1
	}
	if c.hasSelector {
		s += 1
	}
	if c.isFallback {
		s -= 1
	}
	return s
}

func deduplicate(candidates []candidate) []candidate {
	best := make(map[string]candidate)
	for _, c := range candidates {
		existing, ok := best[c.url]
		if !ok {
			best[c.url] = c
			continue
		}
		if c.score > existing.score || (c.score == existing.score && c.namespace+"/"+c.name < existing.namespace+"/"+existing.name) {
			best[c.url] = c
		}
	}
	out := make([]candidate, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	return out
}

func sortAndCap(candidates []candidate, n int) []candidate {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].namespace+"/"+candidates[i].name < candidates[j].namespace+"/"+candidates[j].name
	})
	if len(candidates) > n {
		return candidates[:n]
	}
	return candidates
}

// probe performs a single GET to url, checking for HTTP 200 and valid JSON.
// Returns (statusCode, networkErr, err). networkErr=true means a transport-level failure.
func (d *PrometheusDiscoverer) probe(ctx context.Context, url string) (int, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("User-Agent", "valiant-prometheus-discovery/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, true, err
	}
	defer resp.Body.Close()

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return resp.StatusCode, false, fmt.Errorf("invalid JSON body: %w", err)
	}
	return resp.StatusCode, false, nil
}

// schemeFallbackStatuses are HTTP statuses that warrant trying the alternate scheme.
var schemeFallbackStatuses = map[int]bool{
	301: true, 302: true, 307: true, 308: true, 403: true,
}

// alternateScheme swaps http↔https in a base URL.
func alternateScheme(baseURL string) string {
	if strings.HasPrefix(baseURL, "https://") {
		return "http://" + strings.TrimPrefix(baseURL, "https://")
	}
	return "https://" + strings.TrimPrefix(baseURL, "http://")
}

// tryScheme probes baseURL/api/v1/status/buildinfo, retrying once on transient errors.
// Returns (validatedBaseURL, shouldTryAlternateScheme).
func (d *PrometheusDiscoverer) tryScheme(ctx context.Context, baseURL, label string) (string, bool) {
	target := baseURL + "/api/v1/status/buildinfo"
	status, networkErr, err := d.probe(ctx, target)

	// retry once on transient network errors
	if networkErr {
		status, networkErr, err = d.probe(ctx, target)
	}

	if networkErr || err != nil {
		log.Printf("INFO: Prometheus discovery: candidate %s %s network error: %v", label, baseURL, err)
		return "", true // network error → try alternate scheme
	}
	if status == 200 {
		return baseURL, false
	}
	if schemeFallbackStatuses[status] {
		log.Printf("WARN: Prometheus discovery: candidate %s %s rejected: status=%d → trying alternate scheme", label, baseURL, status)
		return "", true
	}
	log.Printf("WARN: Prometheus discovery: candidate %s %s rejected: status=%d", label, baseURL, status)
	return "", false
}

// tryCandidate tries the candidate's primary scheme then the alternate if warranted.
// Returns the validated base URL, or "" if both fail.
func (d *PrometheusDiscoverer) tryCandidate(ctx context.Context, c candidate) string {
	label := c.namespace + "/" + c.name
	if url, shouldFallback := d.tryScheme(ctx, c.url, label); url != "" {
		return url
	} else if shouldFallback {
		alt := alternateScheme(c.url)
		if url, _ := d.tryScheme(ctx, alt, label); url != "" {
			return url
		}
	}
	return ""
}

// validate runs candidates through parallel validation (up to 3 concurrently).
// High-confidence candidates (score >= shortCircuitScore) are tried alone first.
// Jitter is applied once before the first attempt.
func (d *PrometheusDiscoverer) validate(ctx context.Context, candidates []candidate) (string, error) {
	if len(candidates) == 0 {
		return "", ErrNoPrometheusFound
	}

	// jitter once — prevents thundering herd on multi-instance startup
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

	// short-circuit: high-confidence top candidate tried alone first
	if candidates[0].score >= shortCircuitScore {
		if url := d.tryCandidate(ctx, candidates[0]); url != "" {
			log.Printf("INFO: Prometheus discovery: selected endpoint %s (short-circuit, score=%d)", url, candidates[0].score)
			return url, nil
		}
		candidates = candidates[1:]
		if len(candidates) == 0 {
			return "", ErrNoPrometheusFound
		}
	}

	// parallel validation of up to 3 remaining candidates
	batch := candidates
	if len(batch) > 3 {
		batch = batch[:3]
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	successCh := make(chan string, len(batch))
	var wg sync.WaitGroup

	for _, c := range batch {
		wg.Add(1)
		go func(c candidate) {
			defer wg.Done()
			if url := d.tryCandidate(ctx, c); url != "" {
				select {
				case successCh <- url:
					cancel()
				default:
				}
			}
		}(c)
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case url := <-successCh:
		log.Printf("INFO: Prometheus discovery: selected endpoint %s", url)
		return url, nil
	case <-doneCh:
		return "", ErrNoPrometheusFound
	}
}

// collectNamespaces lists all reachable namespaces, falling back to cfg or ["default"] on RBAC failure.
func (d *PrometheusDiscoverer) collectNamespaces(ctx context.Context) []string {
	nsList, err := d.k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		fallback := d.cfg.Kubernetes.Namespaces
		if len(fallback) == 0 {
			fallback = []string{"default"}
			log.Printf("WARN: Prometheus discovery: namespace listing denied and no namespaces configured, falling back to [\"default\"]")
		} else {
			log.Printf("WARN: Prometheus discovery: cluster-wide namespace listing denied, falling back to configured namespaces: %v", fallback)
		}
		return fallback
	}
	names := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		names = append(names, ns.Name)
	}
	return names
}

// collectServices queries each namespace for Services with Prometheus labels.
// Results are de-duplicated by namespace/name across both label selectors.
func (d *PrometheusDiscoverer) collectServices(ctx context.Context, namespaces []string) []corev1.Service {
	labelSelectors := []string{
		"app.kubernetes.io/name=prometheus",
		"app=prometheus",
	}
	seen := make(map[string]bool)
	var all []corev1.Service
	for _, ns := range namespaces {
		for _, selector := range labelSelectors {
			list, err := d.k8sClient.CoreV1().Services(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				continue
			}
			for _, svc := range list.Items {
				key := svc.Namespace + "/" + svc.Name
				if !seen[key] {
					seen[key] = true
					all = append(all, svc)
				}
			}
		}
	}
	return all
}

// toCandidates converts pre-filtered services into candidates with port and URL selected.
func toCandidates(svcs []corev1.Service) []candidate {
	var out []candidate
	for _, svc := range svcs {
		p, pName, ok, fallback := selectPort(svc.Spec.Ports)
		if !ok {
			continue
		}
		out = append(out, candidate{
			namespace:   svc.Namespace,
			name:        svc.Name,
			clusterIP:   svc.Spec.ClusterIP,
			port:        p,
			portName:    pName,
			url:         buildURL(svc.Spec.ClusterIP, p),
			isFallback:  fallback,
			labels:      svc.Labels,
			hasSelector: len(svc.Spec.Selector) > 0,
		})
	}
	return out
}

// Discover runs the full pipeline and returns the first validated Prometheus-compatible URL.
func (d *PrometheusDiscoverer) Discover(ctx context.Context) (string, error) {
	namespaces := d.collectNamespaces(ctx)
	log.Printf("INFO: Prometheus discovery: querying %d namespace(s)", len(namespaces))

	svcs := d.collectServices(ctx, namespaces)
	filtered := preFilter(svcs)
	log.Printf("INFO: Prometheus discovery candidates after pre-filtering: %d", len(filtered))

	if len(filtered) == 0 {
		log.Printf("INFO: Prometheus discovery: no candidates found after filtering")
		return "", ErrNoPrometheusFound
	}

	candidates := toCandidates(filtered)
	for i := range candidates {
		candidates[i].score = scoreCandidate(candidates[i])
	}

	candidates = deduplicate(candidates)
	total := len(candidates)
	log.Printf("INFO: Prometheus discovery: total candidates after dedup = %d", total)

	candidates = sortAndCap(candidates, maxCandidates)

	log.Printf("INFO: Ranked candidates:")
	for i, c := range candidates {
		log.Printf("  %d. %s/%s  score=%d  url=%s", i+1, c.namespace, c.name, c.score, c.url)
	}

	log.Printf("INFO: Prometheus discovery: validating top %d of %d candidates", len(candidates), total)

	url, err := d.validate(ctx, candidates)
	if err != nil {
		return "", err
	}

	if total > 1 {
		log.Printf("WARN: Multiple valid Prometheus candidates detected, using highest ranked. Consider setting prometheus.url explicitly to suppress this warning.")
	}

	return url, nil
}
