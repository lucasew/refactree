package fuzzy

import "testing"

func TestSessionNetwork(t *testing.T) {
	on := true
	off := false
	p := Project{Isolate: IsolateConfig{SetupNetwork: &on, CheckNetwork: &off}}
	if sessionNetwork(Options{Offline: true}, p) {
		t.Fatal("offline must disable network")
	}
	if !sessionNetwork(Options{Mode: ModePrefetch}, p) {
		t.Fatal("prefetch should follow setup_network")
	}
	if !sessionNetwork(Options{Mode: ModeRun}, p) {
		t.Fatal("run should enable network when setup_network is on")
	}
	p.Isolate.SetupNetwork = &off
	if sessionNetwork(Options{Mode: ModeRun}, p) {
		t.Fatal("run should disable network when both flags are off")
	}
	p.Isolate.CheckNetwork = &on
	if !sessionNetwork(Options{Mode: ModeRun}, p) {
		t.Fatal("run should enable network when check_network is on")
	}
	if sessionNetwork(Options{Mode: ModePrefetch}, p) {
		t.Fatal("prefetch should ignore check_network")
	}
}
