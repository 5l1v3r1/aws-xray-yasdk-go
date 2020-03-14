package xrayhttp

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/aws-xray-yasdk-go/xray"
	"github.com/shogo82148/aws-xray-yasdk-go/xray/schema"
)

func ignoreVariableFieldFunc(in *schema.Segment) *schema.Segment {
	out := *in
	out.ID = ""
	out.TraceID = ""
	out.ParentID = ""
	out.StartTime = 0
	out.EndTime = 0
	out.Subsegments = nil
	for _, sub := range in.Subsegments {
		out.Subsegments = append(out.Subsegments, ignoreVariableFieldFunc(sub))
	}
	return &out
}

// some fields change every execution, ignore them.
var ignoreVariableField = cmp.Transformer("Segment", ignoreVariableFieldFunc)

func TestClient(t *testing.T) {
	ctx, td := xray.NewTestDaemon(nil)
	defer td.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("hello")); err != nil {
			panic(err)
		}
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	func() {
		client := Client(nil)
		ctx, root := xray.BeginSegment(ctx, "test")
		defer root.Close()
		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		req = req.WithContext(ctx)
		req.Host = "example.com"
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal()
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal()
		}
		if string(data) != "hello" {
			t.Errorf("want %q, got %q", "hello", string(data))
		}
	}()

	got, err := td.Recv()
	if err != nil {
		t.Fatal(err)
	}
	want := &schema.Segment{
		Name: "test",
		Subsegments: []*schema.Segment{
			{
				Name:      "example.com",
				Namespace: "remote",
				HTTP: &schema.HTTP{
					Request: &schema.HTTPRequest{
						Method: http.MethodGet,
						URL:    ts.URL,
					},
					Response: &schema.HTTPResponse{
						Status:        http.StatusOK,
						ContentLength: 5,
					},
				},
				Subsegments: []*schema.Segment{
					{
						Name: "connect",
						Subsegments: []*schema.Segment{
							{
								Name: "dial",
								Metadata: map[string]interface{}{
									"http": map[string]interface{}{
										"dial": map[string]interface{}{
											"network": "tcp",
											"address": u.Host,
										},
									},
								},
							},
						},
					},
					{Name: "request"},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got, ignoreVariableField); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestClient_TLS(t *testing.T) {
	ctx, td := xray.NewTestDaemon(nil)
	defer td.Close()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("hello")); err != nil {
			panic(err)
		}
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	func() {
		client := Client(ts.Client())
		ctx, root := xray.BeginSegment(ctx, "test")
		defer root.Close()
		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		req = req.WithContext(ctx)
		req.Host = "example.com"
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal()
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal()
		}
		if string(data) != "hello" {
			t.Errorf("want %q, got %q", "hello", string(data))
		}
	}()

	got, err := td.Recv()
	if err != nil {
		t.Fatal(err)
	}
	want := &schema.Segment{
		Name: "test",
		Subsegments: []*schema.Segment{
			{
				Name:      "example.com",
				Namespace: "remote",
				HTTP: &schema.HTTP{
					Request: &schema.HTTPRequest{
						Method: http.MethodGet,
						URL:    ts.URL,
					},
					Response: &schema.HTTPResponse{
						Status:        http.StatusOK,
						ContentLength: 5,
					},
				},
				Subsegments: []*schema.Segment{
					{
						Name: "connect",
						Subsegments: []*schema.Segment{
							{
								Name: "dial",
								Metadata: map[string]interface{}{
									"http": map[string]interface{}{
										"dial": map[string]interface{}{
											"network": "tcp",
											"address": u.Host,
										},
									},
								},
							},
							{Name: "tls"},
						},
					},
					{Name: "request"},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got, ignoreVariableField); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestClient_DNS(t *testing.T) {
	ctx, td := xray.NewTestDaemon(nil)
	defer td.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("hello")); err != nil {
			panic(err)
		}
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	u.Host = net.JoinHostPort("loopback.shogo82148.com", u.Port())

	// Specify IP version to avoid falling back
	addr := u.Hostname()
	network := "tcp6"
	if net.ParseIP(addr).To4() != nil {
		network = "tcp4"
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return new(net.Dialer).DialContext(ctx, network, addr)
			},
		},
	}

	func() {
		client := Client(client)
		ctx, root := xray.BeginSegment(ctx, "test")
		defer root.Close()
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		req = req.WithContext(ctx)
		req.Host = "example.com"
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello" {
			t.Errorf("want %q, got %q", "hello", string(data))
		}
	}()

	got, err := td.Recv()
	if err != nil {
		t.Fatal(err)
	}
	want := &schema.Segment{
		Name: "test",
		Subsegments: []*schema.Segment{
			{
				Name:      "example.com",
				Namespace: "remote",
				HTTP: &schema.HTTP{
					Request: &schema.HTTPRequest{
						Method: http.MethodGet,
						URL:    u.String(),
					},
					Response: &schema.HTTPResponse{
						Status:        http.StatusOK,
						ContentLength: 5,
					},
				},
				Subsegments: []*schema.Segment{
					{
						Name: "connect",
						Subsegments: []*schema.Segment{
							{
								Name: "dns",
								Metadata: map[string]interface{}{
									"http": map[string]interface{}{
										"dns": map[string]interface{}{
											"addresses": []interface{}{addr},
											"coalesced": false,
										},
									},
								},
							},
							{
								Name: "dial",
								Metadata: map[string]interface{}{
									"http": map[string]interface{}{
										"dial": map[string]interface{}{
											"network": network,
											"address": net.JoinHostPort(addr, u.Port()),
										},
									},
								},
							},
						},
					},
					{Name: "request"},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got, ignoreVariableField); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}
