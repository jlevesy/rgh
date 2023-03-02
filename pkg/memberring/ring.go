package memberring

import (
	"errors"
	"hash/fnv"
	"math"
	"sort"
	"sync"

	"github.com/hashicorp/memberlist"
	"golang.org/x/exp/slices"
)

var (
	errEmptyRing   = errors.New("ring is empty")
	errNoNodeFound = errors.New("no node is found")
)

const defaultWeight = 1

type ringEntry struct {
	serverName string
	bound      uint32
}

type ring struct {
	mu      sync.RWMutex
	entries []ringEntry
}

func (r *ring) init(nodes []*memberlist.Node) error {
	for _, member := range nodes {
		if err := r.addNode(member); err != nil {
			return err
		}
	}

	return nil
}

func (r *ring) getNodeName(key []byte) (string, error) {
	hash, err := hash(key)
	if err != nil {
		return "", err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.entries) == 0 {
		return "", errEmptyRing
	}

	idx := sort.Search(len(r.entries), func(x int) bool {
		return r.entries[x].bound > hash
	})

	if idx == len(r.entries) {
		idx = 0
	}

	return r.entries[idx].serverName, nil
}

func (r *ring) NotifyJoin(n *memberlist.Node) {
	_ = r.addNode(n)
}

func (r *ring) NotifyLeave(n *memberlist.Node) {
	_ = r.removeNode(n)
}

func (r *ring) NotifyUpdate(_ *memberlist.Node) {}

func (r *ring) addNode(n *memberlist.Node) error {
	if !isMemberNode(n) {
		return nil
	}

	if r.isRegistered(n) {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = computeRing(
		append(
			r.entries,
			ringEntry{serverName: n.Name},
		),
	)

	return nil
}

func computeRing(entries []ringEntry) []ringEntry {
	var (
		interval = float64(math.MaxUint32 / len(entries))
		ring     = make([]ringEntry, len(entries))

		current float64
	)

	for i, entry := range entries {
		current += interval

		ring[i] = ringEntry{
			serverName: entry.serverName,
			bound:      uint32(math.Round(current)),
		}
	}

	return ring
}

func (r *ring) removeNode(n *memberlist.Node) error {
	if !isMemberNode(n) {
		return nil
	}

	if !r.isRegistered(n) {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := slices.IndexFunc(r.entries, func(e ringEntry) bool {
		return e.serverName == n.Name
	})

	if idx == -1 {
		// TODO(jly): this is bad, this should not happen
		return nil
	}

	r.entries = slices.Delete(r.entries, idx, idx+1)

	return nil
}

func (r *ring) isRegistered(n *memberlist.Node) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, e := range r.entries {
		if e.serverName == n.Name {
			return true
		}
	}

	return false
}

func isMemberNode(n *memberlist.Node) bool {
	return len(n.Meta) >= 1 && Role(n.Meta[0]) == RoleMember
}

func hash(key []byte) (uint32, error) {
	hasher := fnv.New32a()
	_, err := hasher.Write(key)
	return hasher.Sum32(), err
}
