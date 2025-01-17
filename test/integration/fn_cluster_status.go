// +build integration

/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package integration

import (
	"fmt"
	"testing"
	"time"

	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/minikube/pkg/util/retry"
	"k8s.io/minikube/test/integration/util"
)

func testClusterStatus(t *testing.T) {
	p := profileName(t)
	kr := util.NewKubectlRunner(t, p)
	cs := api.ComponentStatusList{}

	healthy := func() error {
		t.Log("Checking if cluster is healthy.")
		if err := kr.RunCommandParseOutput([]string{"get", "cs"}, &cs); err != nil {
			return err
		}
		for _, i := range cs.Items {
			status := api.ConditionFalse
			for _, c := range i.Conditions {
				if c.Type != api.ComponentHealthy {
					continue
				}
				status = c.Status
			}
			if status != api.ConditionTrue {
				err := fmt.Errorf("component %s is not Healthy! Status: %s", i.GetName(), status)
				t.Logf("Retrying, %v", err)
				return err
			}
		}
		return nil
	}

	if err := retry.Expo(healthy, 500*time.Millisecond, time.Minute); err != nil {
		t.Errorf("Cluster is not healthy: %v", err)
	}
}
