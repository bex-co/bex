/*
Copyright 2026.

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

package egressmeter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type counterLossState struct {
	Events   uint64 `json:"events"`
	LastUnix int64  `json:"lastUnix"`
}

func readCounterLossState(path string) (counterLossState, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return counterLossState{}, nil
	}
	if err != nil {
		return counterLossState{}, err
	}
	var state counterLossState
	if err := json.Unmarshal(raw, &state); err != nil {
		return counterLossState{}, fmt.Errorf("decode counter-loss state: %w", err)
	}
	if state.LastUnix < 0 || (state.Events == 0 && state.LastUnix != 0) || (state.Events > 0 && state.LastUnix == 0) {
		return counterLossState{}, fmt.Errorf("invalid counter-loss state: events=%d lastUnix=%d", state.Events, state.LastUnix)
	}
	return state, nil
}

func writeCounterLossState(path string, state counterLossState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".loss-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(state); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (m *Meter) recordCounterLoss() error {
	m.lossMu.Lock()
	defer m.lossMu.Unlock()
	lastUnix := time.Now().Unix()
	if previous := m.lastLossUnix.Load(); lastUnix < previous {
		lastUnix = previous
	}
	state := counterLossState{Events: m.lossEvents.Load() + 1, LastUnix: lastUnix}
	if err := writeCounterLossState(m.config.LossStatePath, state); err != nil {
		return err
	}
	m.lossEvents.Store(state.Events)
	m.lastLossUnix.Store(state.LastUnix)
	return nil
}
