package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const activationStoreFile = "activations.json"

var activationMu sync.Mutex

type EnvActivationEvent struct {
	At       int64  `json:"at"`
	Provider string `json:"provider"`
	EnvName  string `json:"env_name"`
}

type activationStore struct {
	Providers map[string][]EnvActivationEvent `json:"providers"`
}

func activationStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, mcpStoreDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, activationStoreFile), nil
}

func loadActivationStore() (activationStore, error) {
	path, err := activationStorePath()
	if err != nil {
		return activationStore{}, err
	}

	store := activationStore{Providers: map[string][]EnvActivationEvent{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return activationStore{}, err
	}
	if len(data) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(data, &store); err != nil {
		return activationStore{}, err
	}
	if store.Providers == nil {
		store.Providers = map[string][]EnvActivationEvent{}
	}

	normalizeActivationStore(&store)
	return store, nil
}

func saveActivationStore(store activationStore) error {
	path, err := activationStorePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func normalizeActivationStore(store *activationStore) {
	if store == nil {
		return
	}
	if store.Providers == nil {
		store.Providers = map[string][]EnvActivationEvent{}
	}

	for provider, events := range store.Providers {
		p := normalizeProvider(provider)
		if p == "" {
			delete(store.Providers, provider)
			continue
		}

		next := make([]EnvActivationEvent, 0, len(events))
		for _, e := range events {
			env := strings.TrimSpace(e.EnvName)
			if env == "" {
				continue
			}
			at := e.At
			if at <= 0 {
				continue
			}
			next = append(next, EnvActivationEvent{At: at, Provider: p, EnvName: env})
		}

		sort.SliceStable(next, func(i, j int) bool { return next[i].At < next[j].At })
		next = compactActivationEvents(next)

		// 只保留最近 N 条，避免无限增长
		const keepMax = 5000
		if len(next) > keepMax {
			next = next[len(next)-keepMax:]
		}

		store.Providers[p] = next
		if p != provider {
			delete(store.Providers, provider)
		}
	}
}

func compactActivationEvents(events []EnvActivationEvent) []EnvActivationEvent {
	if len(events) == 0 {
		return events
	}
	out := make([]EnvActivationEvent, 0, len(events))
	for _, e := range events {
		if len(out) == 0 {
			out = append(out, e)
			continue
		}
		last := out[len(out)-1]
		if last.EnvName == e.EnvName {
			continue
		}
		out = append(out, e)
	}
	return out
}

func normalizeProvider(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		return "claude"
	}
	switch p {
	case "claude", "codex", "gemini":
		return p
	default:
		return ""
	}
}

func RecordEnvActivation(provider, envName string, at time.Time) error {
	p := normalizeProvider(provider)
	env := strings.TrimSpace(envName)
	if p == "" || env == "" {
		return nil
	}

	activationMu.Lock()
	defer activationMu.Unlock()

	store, err := loadActivationStore()
	if err != nil {
		return err
	}

	if store.Providers == nil {
		store.Providers = map[string][]EnvActivationEvent{}
	}

	events := store.Providers[p]
	event := EnvActivationEvent{At: at.Unix(), Provider: p, EnvName: env}

	if len(events) > 0 && events[len(events)-1].EnvName == env {
		return nil
	}

	events = append(events, event)
	store.Providers[p] = events
	normalizeActivationStore(&store)

	return saveActivationStore(store)
}

func LoadEnvActivations() (map[string][]EnvActivationEvent, error) {
	activationMu.Lock()
	defer activationMu.Unlock()

	store, err := loadActivationStore()
	if err != nil {
		return nil, err
	}
	if store.Providers == nil {
		store.Providers = map[string][]EnvActivationEvent{}
	}

	clone := make(map[string][]EnvActivationEvent, len(store.Providers))
	for k, v := range store.Providers {
		cp := make([]EnvActivationEvent, len(v))
		copy(cp, v)
		clone[k] = cp
	}
	return clone, nil
}

