package feature

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

type probeAdapter struct {
	agent.Adapter
	id    string
	probe func() agent.Agent
}

func (a probeAdapter) ID() string          { return a.id }
func (a probeAdapter) Detect() agent.Agent { return a.probe() }

func TestDetectionRefreshSharesSnapshotsAndPreservesFailedEvidence(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	a := probeAdapter{id: "a", probe: func() agent.Agent {
		n := calls.Add(1)
		if n == 2 {
			close(started)
			<-release
		}
		if n == 3 {
			return agent.Agent{ID: "a", DetectionError: "probe failed"}
		}
		return agent.Agent{ID: "a", Installed: true, Version: time.Duration(n).String(), ReadDirs: []string{"original"}}
	}}
	var other atomic.Int32
	b := probeAdapter{id: "b", probe: func() agent.Agent {
		return agent.Agent{ID: "b", Installed: other.Add(1) > 1}
	}}
	d := &Deps{Adapters: []agent.Adapter{a, b}}
	initial := d.DetectAgents(false)
	initial.Agents[0].ReadDirs[0] = "caller mutation"
	if d.Agents()[0].ReadDirs[0] != "original" || calls.Load() != 1 {
		t.Fatal("cache was mutated or reprobed")
	}
	done := make(chan agent.Detection, 1)
	go func() { done <- d.DetectAgents(true) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}
	var readers sync.WaitGroup
	for range 16 {
		readers.Go(func() { snapshot := d.Agents(); snapshot[0].ReadDirs[0] = "private" })
	}
	read := make(chan struct{})
	go func() { readers.Wait(); close(read) }()
	select {
	case <-read:
	case <-time.After(time.Second):
		t.Fatal("cached readers blocked on a refresh")
	}
	unblock.Do(func() { close(release) })
	fresh := <-done
	if fresh.Revision != 2 || !fresh.Agents[1].Installed || calls.Load() != 2 || other.Load() != 2 {
		t.Fatalf("fresh snapshot = %+v", fresh)
	}
	failed := d.DetectAgents(true)
	if failed.Revision != 3 || failed.Agents[0].Version != fresh.Agents[0].Version || !failed.Agents[0].Installed || failed.Agents[0].DetectionError == "" || !failed.Agents[1].Installed {
		t.Fatalf("partial failure erased prior or healthy evidence: %+v", failed)
	}
	recovered := d.DetectAgents(true)
	if recovered.Agents[0].DetectionError != "" || recovered.Revision != 4 {
		t.Fatalf("recovery retained stale failure: %+v", recovered)
	}
}
