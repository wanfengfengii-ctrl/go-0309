package domain

import "sort"

// CanonicalKey is the fully qualified sort key used to order diagnostics and
// audit records uniformly: tunnel, chainage, cycle, rock zone, surface unit,
// spray layer, spray band and generation. Every field is optional; empty fields
// sort before populated ones so that partial records remain deterministic.
type CanonicalKey struct {
	Tunnel     string
	Chainage   int64
	Cycle      int64
	Zone       string
	Unit       string
	Layer      int64
	Band       string
	Generation int64
}

// Less compares two canonical keys field by field.
func Less(a, b CanonicalKey) bool {
	if a.Tunnel != b.Tunnel {
		return a.Tunnel < b.Tunnel
	}
	if a.Chainage != b.Chainage {
		return a.Chainage < b.Chainage
	}
	if a.Cycle != b.Cycle {
		return a.Cycle < b.Cycle
	}
	if a.Zone != b.Zone {
		return a.Zone < b.Zone
	}
	if a.Unit != b.Unit {
		return a.Unit < b.Unit
	}
	if a.Layer != b.Layer {
		return a.Layer < b.Layer
	}
	if a.Band != b.Band {
		return a.Band < b.Band
	}
	return a.Generation < b.Generation
}

// SortReasons orders diagnostics by their key (then code and message) so that
// error detail lists are stable across runs and goroutines.
func SortReasons(rs []Reason) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Key != rs[j].Key {
			return rs[i].Key < rs[j].Key
		}
		if rs[i].Code != rs[j].Code {
			return rs[i].Code < rs[j].Code
		}
		return rs[i].Message < rs[j].Message
	})
}
