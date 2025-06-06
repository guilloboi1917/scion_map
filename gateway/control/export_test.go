// Copyright 2020 Anapaya Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package control

import (
	"context"

	"golang.org/x/sync/errgroup"
)

var (
	BuildSessionConfigs     = buildSessionConfigs
	ComputeDiff             = computeDiff
	NewPathPolForEnteringAS = newPathPolForEnteringAS
	NewPrefixWatcher        = newPrefixWatcher

	CopyPathPolicy     = copyPathPolicy
	BuildRoutingChains = buildRoutingChains
)

type (
	ConjunctionPathPol = conjuctionPathPol
	Diff               = diff
)

func (w *GatewayWatcher) RunOnce(ctx context.Context) {
	w.run(ctx)
}

func (w *GatewayWatcher) RunAllPrefixWatchersOnceForTest(ctx context.Context) error {
	var eg errgroup.Group
	for _, wi := range w.currentWatchers {
		wi.prefixWatcher.resetRunMarker()
		eg.Go(func() error {
			return wi.prefixWatcher.Run(ctx)
		})
	}
	return eg.Wait()
}

func (w *prefixWatcher) resetRunMarker() {
	w.runMarkerLock.Lock()
	defer w.runMarkerLock.Unlock()
	w.runMarker = false
}
