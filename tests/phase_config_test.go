package tests

import "testing"

const (
	phaseACLAuth                = "acl_auth"
	phaseAOF                    = "aof"
	phaseBase                   = "base"
	phaseCustomActiveExpiry     = "custom_active_expiry"
	phaseCustomGracefulShutdown = "custom_graceful_shutdown"
	phaseCustomHashes           = "custom_hashes"
	phaseCustomKeyManagement    = "custom_keymgmt"
	phaseCustomServerCommands   = "custom_servercmds"
	phaseCustomSets             = "custom_sets"
	phaseCustomSharding         = "custom_sharding"
	phaseCustomStringOperations = "custom_stringops"
	phaseCustomTTL              = "custom_ttl"
	phaseGeospatial             = "geospatial"
	phaseLists                  = "lists"
	phaseOptimisticLocking      = "optimistic_locking"
	phasePubSub                 = "pubsub"
	phaseRDB                    = "rdb"
	phaseReplication            = "replication"
	phaseSortedSets             = "sortedset"
	phaseStreams                = "streams"
	phaseTransactions           = "transactions"
)

var enabledPhases = map[string]bool{
	phaseACLAuth:                false,
	phaseAOF:                    true,
	phaseBase:                   true,
	phaseCustomActiveExpiry:     false,
	phaseCustomGracefulShutdown: false,
	phaseCustomHashes:           false,
	phaseCustomKeyManagement:    false,
	phaseCustomServerCommands:   false,
	phaseCustomSets:             false,
	phaseCustomSharding:         false,
	phaseCustomStringOperations: false,
	phaseCustomTTL:              false,
	phaseGeospatial:             true,
	phaseLists:                  true,
	phaseOptimisticLocking:      true,
	phasePubSub:                 true,
	phaseRDB:                    true,
	phaseReplication:            false,
	phaseSortedSets:             true,
	phaseStreams:                false,
	phaseTransactions:           true,
}

func requirePhase(t *testing.T, phase string) {
	t.Helper()
	if !enabledPhases[phase] {
		t.Skipf("skipping %s tests; enable the phase in enabledPhases", phase)
	}
}
