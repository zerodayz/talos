// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// InitialNetworkSetup represents the task for setting up the initial network.
type InitialNetworkSetup struct{}

// NewInitialNetworkSetupTask initializes and returns an InitialNetworkSetup task.
func NewInitialNetworkSetupTask() phase.Task {
	return &InitialNetworkSetup{}
}

// TaskFunc returns the runtime function.
func (task *InitialNetworkSetup) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *InitialNetworkSetup) runtime(r runtime.Runtime) (err error) {
	nwd, err := networkd.New(r.Config())
	if err != nil {
		return err
	}

	if err = nwd.Configure(); err != nil {
		return err
	}

	return nil
}
