// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"

	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/pkg/client"
)

// ApidReadyAssertion checks whether apid is responsive on all the nodes.
func ApidReadyAssertion(ctx context.Context, cluster provision.ClusterAccess) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	nodes := make([]string, 0, len(cluster.Info().Nodes))

	for _, node := range cluster.Info().Nodes {
		nodes = append(nodes, node.PrivateIP.String())
	}

	nodesCtx := client.WithNodes(ctx, nodes...)

	_, err = cli.Version(nodesCtx)

	return err
}
