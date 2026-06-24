package sdkapp

import (
	"testing"
	"time"
)

func TestMDNSDiscoveryPreventsOverlapAndAppliesCooldown(t *testing.T) {
	mdnsDiscoveryState.Lock()
	mdnsDiscoveryState.running = false
	mdnsDiscoveryState.lastFinished = time.Time{}
	mdnsDiscoveryState.Unlock()
	t.Cleanup(func() {
		mdnsDiscoveryState.Lock()
		mdnsDiscoveryState.running = false
		mdnsDiscoveryState.lastFinished = time.Time{}
		mdnsDiscoveryState.Unlock()
	})

	started := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
	if !beginMDNSDiscovery(started) {
		t.Fatal("first mDNS discovery was rejected")
	}
	if beginMDNSDiscovery(started.Add(time.Second)) {
		t.Fatal("overlapping mDNS discovery was allowed")
	}

	finished := started.Add(5 * time.Second)
	finishMDNSDiscovery(finished)
	if beginMDNSDiscovery(finished.Add(mdnsDiscoveryCooldown - time.Second)) {
		t.Fatal("mDNS discovery ignored its cooldown")
	}
	if !beginMDNSDiscovery(finished.Add(mdnsDiscoveryCooldown)) {
		t.Fatal("mDNS discovery did not resume after its cooldown")
	}
}
