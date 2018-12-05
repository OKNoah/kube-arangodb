//
// DISCLAIMER
//
// Copyright 2018 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//

package tests

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	driver "github.com/arangodb/go-driver"
	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1alpha"
	"github.com/arangodb/kube-arangodb/pkg/client"
	"github.com/arangodb/kube-arangodb/pkg/util"
	"github.com/dchest/uniuri"
	"github.com/stretchr/testify/require"
)

type clusterEndpointsResponse struct {
	Endpoints []clusterEndpoint `json:"endpoints,omitempty"`
}

type clusterEndpoint struct {
	Endpoint string `json:"endpoint,omitempty"`
}

func getAdvertisedEndpoints(ctx context.Context, c driver.Client) ([]string, error) {
	con := c.Connection()
	req, err := con.NewRequest("GET", "_api/cluster/endpoints")
	if err != nil {
		return nil, err
	}
	resp, err := con.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	var data clusterEndpointsResponse
	if err := resp.ParseBody("", &data); err != nil {
		return nil, err
	}

	endpoints := []string{}
	for _, endpoint := range data.Endpoints {
		endpoints = append(endpoints, endpoint.Endpoint)
	}
	return endpoints, nil
}

func TestAdvertisedEndpointCluster(t *testing.T) {
	c := client.MustNewInCluster()
	kubecli := mustNewKubeClient(t)
	ns := getNamespace(t)

	depl := newDeployment("test-adv-ep-" + uniuri.NewLen(4))
	depl.Spec.Mode = api.NewMode(api.DeploymentModeCluster)
	depl.Spec.ExternalAccess.AdvertisedEndpoint = util.NewString("tcp://example.com")
	depl.Spec.SetDefaults(depl.GetName())

	// Create deployment
	apiObject, err := c.DatabaseV1alpha().ArangoDeployments(ns).Create(depl)
	if err != nil {
		t.Fatalf("Create deployment failed: %v", err)
	}
	defer deferedCleanupDeployment(c, depl.GetName(), ns)

	// Wait for deployment to be ready
	_, err = waitUntilDeployment(c, depl.GetName(), ns, deploymentIsReady())
	require.NoError(t, err, fmt.Sprintf("Deployment not running in time: %v", err))

	// Create a database client
	ctx := context.Background()
	client := mustNewArangodDatabaseClient(ctx, kubecli, apiObject, t, nil)

	expected := []string{
		"tcp://example.com",
		"tcp://example.com",
		"tcp://example.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	endpoints, err := getAdvertisedEndpoints(ctx, client)
	cancel()
	if err != nil {
		t.Fatalf("Could not get advertised endpoints: %v", err)
	}

	require.Equal(t, expected, endpoints, "Expected endpoints: %v, found: %v", expected, endpoints)

	if _, err := updateDeployment(c, depl.GetName(), ns,
		func(spec *api.DeploymentSpec) {
			spec.ExternalAccess.AdvertisedEndpoint = util.NewString("tcp://sub.example.com")
		}); err != nil {
		t.Fatalf("Failed to update the deployment mode: %v", err)
	}

	expected = []string{
		"tcp://sub.example.com",
		"tcp://sub.example.com",
		"tcp://sub.example.com",
	}

	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	op := func(*api.ArangoDeployment) error {
		var endpoints []string
		if endpoints, err = getAdvertisedEndpoints(ctx, client); err != nil {
			return err
		}
		sort.Strings(endpoints)

		if !reflect.DeepEqual(expected, endpoints) {
			return fmt.Errorf("Expected endpoints: %v, found: %v", expected, endpoints)
		}
		return nil
	}

	_, err = waitUntilDeployment(c, depl.GetName(), ns, op)
	require.NoError(t, err, fmt.Sprintf("Deployment not running in time: %v", err))
	cancel()

	removeDeployment(c, depl.GetName(), ns)
}
