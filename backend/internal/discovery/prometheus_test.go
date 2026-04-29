package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"valiant/internal/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeSvc(name, ns, clusterIP string, svcType corev1.ServiceType, ports []corev1.ServicePort, labels, selector map[string]string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec: corev1.ServiceSpec{
			ClusterIP: clusterIP,
			Type:      svcType,
			Ports:     ports,
			Selector:  selector,
		},
	}
}

func makePort(name string, number int32) corev1.ServicePort {
	return corev1.ServicePort{Name: name, Port: number}
}

// --- preFilter ---

func TestPreFilter(t *testing.T) {
	tests := []struct {
		name    string
		input   []corev1.Service
		wantLen int
	}{
		{
			name: "keeps normal ClusterIP service",
			input: []corev1.Service{
				makeSvc("prom", "monitoring", "10.0.0.1", corev1.ServiceTypeClusterIP,
					[]corev1.ServicePort{makePort("http", 9090)}, nil, nil),
			},
			wantLen: 1,
		},
		{
			name:    "removes headless (clusterIP=None)",
			input:   []corev1.Service{makeSvc("prom", "ns", "None", corev1.ServiceTypeClusterIP, nil, nil, nil)},
			wantLen: 0,
		},
		{
			name:    "removes uninitialized (clusterIP empty)",
			input:   []corev1.Service{makeSvc("prom", "ns", "", corev1.ServiceTypeClusterIP, nil, nil, nil)},
			wantLen: 0,
		},
		{
			name:    "removes ExternalName",
			input:   []corev1.Service{makeSvc("prom", "ns", "10.0.0.1", corev1.ServiceTypeExternalName, nil, nil, nil)},
			wantLen: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preFilter(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("preFilter() len=%d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// --- selectPort ---

func TestSelectPort(t *testing.T) {
	tests := []struct {
		name         string
		ports        []corev1.ServicePort
		wantPort     int32
		wantOK       bool
		wantFallback bool
	}{
		{"port 9090 preferred", []corev1.ServicePort{makePort("metrics", 9090)}, 9090, true, false},
		{"named http", []corev1.ServicePort{makePort("http", 8080)}, 8080, true, false},
		{"named web", []corev1.ServicePort{makePort("web", 3000)}, 3000, true, false},
		{"named http-web", []corev1.ServicePort{makePort("http-web", 4000)}, 4000, true, false},
		{"named prometheus", []corev1.ServicePort{makePort("prometheus", 5000)}, 5000, true, false},
		{"single unnamed fallback", []corev1.ServicePort{makePort("other", 8765)}, 8765, true, true},
		{"multi no match rejects", []corev1.ServicePort{makePort("grpc", 9001), makePort("debug", 6060)}, 0, false, false},
		{"empty rejects", nil, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _, ok, fallback := selectPort(tt.ports)
			if ok != tt.wantOK {
				t.Errorf("ok=%v want %v", ok, tt.wantOK)
			}
			if ok && p != tt.wantPort {
				t.Errorf("port=%d want %d", p, tt.wantPort)
			}
			if fallback != tt.wantFallback {
				t.Errorf("fallback=%v want %v", fallback, tt.wantFallback)
			}
		})
	}
}

// --- buildURL ---

func TestBuildURL(t *testing.T) {
	tests := []struct {
		ip   string
		port int32
		want string
	}{
		{"10.0.0.1", 9090, "http://10.0.0.1:9090"},
		{"10.0.0.1", 9091, "https://10.0.0.1:9091"},
		{"10.0.0.1", 8080, "http://10.0.0.1:8080"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s:%d", tt.ip, tt.port), func(t *testing.T) {
			got := buildURL(tt.ip, tt.port)
			if got != tt.want {
				t.Errorf("buildURL=%q want %q", got, tt.want)
			}
		})
	}
}

// --- scoreCandidate ---

func TestScoreCandidate(t *testing.T) {
	tests := []struct {
		name      string
		c         candidate
		wantScore int
	}{
		{
			name: "port 9090 + http name + prometheus in name + monitoring ns + component label + selector",
			c: candidate{
				namespace:   "monitoring",
				name:        "prometheus",
				port:        9090,
				portName:    "http",
				labels:      map[string]string{"app.kubernetes.io/component": "server"},
				hasSelector: true,
			},
			wantScore: 10, // 2+2+2+2+1+1
		},
		{
			name:      "single-port fallback penalty only",
			c:         candidate{namespace: "other", name: "prom", port: 8080, isFallback: true},
			wantScore: -1, // only isFallback=-1, "prom" does not contain "prometheus"
		},
		{
			name: "thanos in openshift-monitoring",
			c: candidate{
				namespace:   "openshift-monitoring",
				name:        "thanos-querier",
				port:        9091,
				portName:    "web",
				hasSelector: true,
			},
			wantScore: 6, // web+2, monitoring+2, thanos+1, selector+1
		},
		{
			name: "prometheus name with fallback port",
			c:    candidate{namespace: "default", name: "prometheus", port: 8888, isFallback: true},
			wantScore: 1, // name+2, fallback-1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreCandidate(tt.c)
			if got != tt.wantScore {
				t.Errorf("scoreCandidate()=%d, want %d", got, tt.wantScore)
			}
		})
	}
}

// --- deduplicate ---

func TestDeduplicate(t *testing.T) {
	candidates := []candidate{
		{namespace: "a", name: "prom1", url: "http://10.0.0.1:9090", score: 3},
		{namespace: "b", name: "prom2", url: "http://10.0.0.1:9090", score: 7},
		{namespace: "c", name: "prom3", url: "http://10.0.0.2:9090", score: 5},
	}
	got := deduplicate(candidates)
	if len(got) != 2 {
		t.Fatalf("deduplicate len=%d want 2", len(got))
	}
	for _, c := range got {
		if c.url == "http://10.0.0.1:9090" && c.score != 7 {
			t.Errorf("dedup kept wrong candidate: score=%d want 7", c.score)
		}
	}
}

// --- sortAndCap ---

func TestSortAndCap(t *testing.T) {
	candidates := []candidate{
		{namespace: "a", name: "z", score: 1},
		{namespace: "a", name: "a", score: 5},
		{namespace: "a", name: "m", score: 5},
		{namespace: "b", name: "x", score: 3},
		{namespace: "c", name: "y", score: 8},
		{namespace: "d", name: "w", score: 2},
	}
	got := sortAndCap(candidates, 3)
	if len(got) != 3 {
		t.Fatalf("sortAndCap len=%d want 3", len(got))
	}
	if got[0].score != 8 {
		t.Errorf("first score=%d want 8", got[0].score)
	}
	if got[1].name != "a" {
		t.Errorf("second should be a/a (alpha tie-break), got %s/%s", got[1].namespace, got[1].name)
	}
}

// --- probe ---

func TestProbe(t *testing.T) {
	d := &PrometheusDiscoverer{httpClient: &http.Client{Timeout: 3 * time.Second}}

	t.Run("200 with valid JSON returns status=200 no error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") != "valiant-prometheus-discovery/1.0" {
				t.Errorf("missing User-Agent header, got %q", r.Header.Get("User-Agent"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"success","data":{}}`)
		}))
		defer srv.Close()

		status, networkErr, err := d.probe(context.Background(), srv.URL+"/api/v1/status/buildinfo")
		if networkErr || err != nil || status != 200 {
			t.Errorf("expected (200, false, nil), got (%d, %v, %v)", status, networkErr, err)
		}
	})

	t.Run("403 returns status=403 no network error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}))
		defer srv.Close()

		status, networkErr, _ := d.probe(context.Background(), srv.URL+"/api/v1/status/buildinfo")
		if networkErr {
			t.Error("expected networkErr=false for HTTP 403")
		}
		if status != 403 {
			t.Errorf("expected status=403, got %d", status)
		}
	})

	t.Run("unreachable returns networkErr=true", func(t *testing.T) {
		_, networkErr, err := d.probe(context.Background(), "http://192.0.2.1:9090/api/v1/status/buildinfo")
		if !networkErr || err == nil {
			t.Errorf("expected networkErr=true and err!=nil, got networkErr=%v err=%v", networkErr, err)
		}
	})
}

// --- tryCandidate ---

func TestTryCandidate(t *testing.T) {
	d := &PrometheusDiscoverer{httpClient: &http.Client{Timeout: 3 * time.Second}}

	t.Run("http succeeds — returns http URL", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()

		c := candidate{namespace: "ns", name: "prom", url: srv.URL}
		got := d.tryCandidate(context.Background(), c)
		if got != srv.URL {
			t.Errorf("tryCandidate=%q want %q", got, srv.URL)
		}
	})

	t.Run("403 triggers scheme fallback, fallback also fails → empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "forbidden", 403)
		}))
		defer srv.Close()
		c := candidate{namespace: "ns", name: "prom", url: srv.URL}
		got := d.tryCandidate(context.Background(), c)
		if got != "" {
			t.Errorf("expected empty URL when both schemes fail, got %q", got)
		}
	})

	t.Run("404 is final, no scheme fallback", func(t *testing.T) {
		httpCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpCount++
			http.Error(w, "not found", 404)
		}))
		defer srv.Close()
		c := candidate{namespace: "ns", name: "prom", url: srv.URL}
		d.tryCandidate(context.Background(), c)
		if httpCount != 1 {
			t.Errorf("expected 1 request for 404 (final), got %d", httpCount)
		}
	})
}

// --- validate ---

func TestValidate(t *testing.T) {
	t.Run("returns validated URL for working candidate", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			fmt.Fprint(w, `{"status":"success"}`)
		}))
		defer srv.Close()

		d := &PrometheusDiscoverer{httpClient: &http.Client{Timeout: 3 * time.Second}}
		got, err := d.validate(context.Background(), []candidate{
			{namespace: "ns", name: "prom", url: srv.URL, score: 5},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != srv.URL {
			t.Errorf("validate=%q want %q", got, srv.URL)
		}
	})

	t.Run("returns ErrNoPrometheusFound when all candidates fail", func(t *testing.T) {
		d := &PrometheusDiscoverer{httpClient: &http.Client{Timeout: 500 * time.Millisecond}}
		_, err := d.validate(context.Background(), []candidate{
			{namespace: "ns", name: "prom", url: "http://192.0.2.1:9090", score: 5},
		})
		if !errors.Is(err, ErrNoPrometheusFound) {
			t.Errorf("expected ErrNoPrometheusFound, got %v", err)
		}
	})

	t.Run("empty candidates returns ErrNoPrometheusFound", func(t *testing.T) {
		d := &PrometheusDiscoverer{httpClient: &http.Client{Timeout: time.Second}}
		_, err := d.validate(context.Background(), nil)
		if !errors.Is(err, ErrNoPrometheusFound) {
			t.Errorf("expected ErrNoPrometheusFound for empty candidates, got %v", err)
		}
	})

	t.Run("short-circuit: score>=7 candidate tried first alone", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(200)
			fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()

		d := &PrometheusDiscoverer{httpClient: &http.Client{Timeout: 3 * time.Second}}
		_, err := d.validate(context.Background(), []candidate{
			{namespace: "ns", name: "prom", url: srv.URL, score: 8},
			{namespace: "ns", name: "other", url: "http://192.0.2.1:9090", score: 2},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected 1 probe call (short-circuit), got %d", callCount)
		}
	})
}

// --- Discover (integration) ---

func TestDiscover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"success"}`)
	}))
	defer srv.Close()

	trimmed := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.SplitN(trimmed, ":", 2)
	ip := parts[0]
	portNum, _ := strconv.ParseInt(parts[1], 10, 32)

	fakeNS := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "monitoring"}}
	fakeSvc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: "monitoring",
			Labels:    map[string]string{"app.kubernetes.io/name": "prometheus"},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: ip,
			Type:      corev1.ServiceTypeClusterIP,
			Ports:     []corev1.ServicePort{{Name: "http", Port: int32(portNum)}},
			Selector:  map[string]string{"app": "prometheus"},
		},
	}

	d := &PrometheusDiscoverer{
		k8sClient:  fake.NewSimpleClientset(&fakeNS, &fakeSvc),
		httpClient: &http.Client{Timeout: 3 * time.Second},
		cfg:        config.Config{},
	}

	got, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	want := fmt.Sprintf("http://%s:%d", ip, portNum)
	if got != want {
		t.Errorf("Discover()=%q want %q", got, want)
	}
}

func TestDiscoverNoServices(t *testing.T) {
	d := &PrometheusDiscoverer{
		k8sClient:  fake.NewSimpleClientset(),
		httpClient: &http.Client{Timeout: time.Second},
		cfg:        config.Config{},
	}
	_, err := d.Discover(context.Background())
	if !errors.Is(err, ErrNoPrometheusFound) {
		t.Errorf("expected ErrNoPrometheusFound, got %v", err)
	}
}
