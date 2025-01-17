// +build integration

/*
Copyright 2019 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// the name of this file starts with z intentionally to make it run last after all other tests
// the intent is to make sure os env proxy settings be done after all other tests.
// for example in the case the test proxy clean up gets killed or fails
package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"net/http"
	"net/url"

	"github.com/elazarl/goproxy"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"k8s.io/minikube/test/integration/util"
)

// setUpProxy runs a local http proxy and sets the env vars for it.
func setUpProxy(t *testing.T) (*http.Server, error) {
	port, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get an open port")
	}

	addr := fmt.Sprintf("localhost:%d", port)
	err = os.Setenv("NO_PROXY", "")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set no proxy env")
	}
	err = os.Setenv("HTTP_PROXY", addr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set http proxy env")
	}

	proxy := goproxy.NewProxyHttpServer()
	srv := &http.Server{Addr: addr, Handler: proxy}
	go func(s *http.Server, t *testing.T) {
		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			t.Errorf("Failed to start http server for proxy mock")
		}
	}(srv, t)
	return srv, nil
}

func TestProxy(t *testing.T) {
	origHP := os.Getenv("HTTP_PROXY")
	origNP := os.Getenv("NO_PROXY")
	p := profileName(t) // profile name
	if isTestNoneDriver(t) {
		// TODO fix this later
		t.Skip("Skipping proxy warning for none")
	}

	srv, err := setUpProxy(t)
	if err != nil {
		t.Fatalf("Failed to set up the test proxy: %s", err)
	}

	// making sure there is no running minikube to avoid https://github.com/kubernetes/minikube/issues/4132
	mk := NewMinikubeRunner(t, p)
	// Clean up after setting up proxy
	defer func(t *testing.T) {
		err = os.Setenv("HTTP_PROXY", origHP)
		if err != nil {
			t.Errorf("Error reverting the HTTP_PROXY env")
		}
		err = os.Setenv("NO_PROXY", origNP)
		if err != nil {
			t.Errorf("Error reverting the NO_PROXY env")
		}

		err := srv.Shutdown(context.TODO()) // shutting down the http proxy after tests
		if err != nil {
			t.Errorf("Error shutting down the http proxy")
		}
		if !isTestNoneDriver(t) {
			mk.TearDown(t)
		}

	}(t)
	t.Run("ProxyConsoleWarnning", testProxyWarning)
	t.Run("ProxyDashboard", testProxyDashboard)
	t.Run("KubeconfigContext", testKubeConfigCurrentCtx)
}

// testProxyWarning checks user is warned correctly about the proxy related env vars
func testProxyWarning(t *testing.T) {
	p := profileName(t) // profile name
	mk := NewMinikubeRunner(t, p)
	stdout, stderr := mk.MustStart("--wait=false")

	msg := "Found network options:"
	if !strings.Contains(stdout, msg) {
		t.Errorf("Proxy wranning (%s) is missing from the output: %s", msg, stderr)
	}

	msg = "You appear to be using a proxy"
	if !strings.Contains(stderr, msg) {
		t.Errorf("Proxy wranning (%s) is missing from the output: %s", msg, stderr)
	}
}

// testProxyDashboard checks if dashboard URL is accessible if proxy is set
func testProxyDashboard(t *testing.T) {
	p := profileName(t) // profile name
	mk := NewMinikubeRunner(t, p)
	cmd, out := mk.RunDaemon("dashboard --url")
	defer func() {
		err := util.KillProcess(cmd.Process.Pid, t)
		if err != nil {
			t.Logf("Failed to kill dashboard command: %v", err)
		}
	}()

	s, err := readLineWithTimeout(out, 180*time.Second)
	if err != nil {
		t.Fatalf("failed to read url: %v", err)
	}

	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("failed to parse %q: %v", s, err)
	}

	resp, err := retryablehttp.Get(u.String())
	if err != nil {
		t.Fatalf("failed get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Unable to read http response body: %v", err)
		}
		t.Errorf("%s returned status code %d, expected %d.\nbody:\n%s", u, resp.StatusCode, http.StatusOK, body)
	}
}

// testKubeConfigCurrentCtx checks weather the current-context is set after star
func testKubeConfigCurrentCtx(t *testing.T) {
	p := profileName(t) // profile name
	kr := util.NewKubectlRunner(t, p)
	ctxAfter, err := kr.RunCommand([]string{"config", "current-context"}, false)
	if err != nil {
		t.Errorf("expected not to get error for kubectl config current-context but got error: %v", err)
	}

	if !strings.Contains(string(ctxAfter), p) {
		t.Errorf("expected kubecontext after start to be %s but got %s", p, ctxAfter)
	}
}
