package store

// FeatureSnapshot — a read-once view of every feature in the store.
//
// v0.15.1 Wave C rev-1 (adjudication finding 2): `tpatch verify` must
// read the feature set EXACTLY ONCE per invocation and answer every
// later question from that capture. The store-level validators it calls
// (`ValidateDependencies`, and the workflow dependency gate) previously
// re-loaded each parent's `status.json` from disk, which both defeated
// the immutability guarantee and re-introduced the silent-drop class
// `ListFeatures` has.
//
// A snapshot is an explicit, injectable substitute for those reads. It
// is deliberately NOT the default: unrelated callers keep loading from
// disk, so no shipped behaviour changes.

import (
	"fmt"
	"os"
	"sort"
)

// FeatureSnapshot carries one status (or one load error) per slug, in
// the slug-sorted order `ListFeatureEntries` returns.
type FeatureSnapshot struct {
	Order  []string
	Status map[string]FeatureStatus
	Errs   map[string]error
}

// NewFeatureSnapshot captures the store via ListFeatureEntries, which —
// unlike ListFeatures — retains unreadable features as error rows
// instead of dropping them.
func NewFeatureSnapshot(s *Store) (*FeatureSnapshot, error) {
	entries, err := s.ListFeatureEntries()
	if err != nil {
		return nil, err
	}
	return NewFeatureSnapshotFromEntries(entries), nil
}

// NewFeatureSnapshotFromEntries builds a snapshot from an already
// captured entry list, so a caller that has one need not read twice.
func NewFeatureSnapshotFromEntries(entries []FeatureEntry) *FeatureSnapshot {
	snap := &FeatureSnapshot{
		Status: make(map[string]FeatureStatus, len(entries)),
		Errs:   make(map[string]error, len(entries)),
	}
	for _, e := range entries {
		snap.Order = append(snap.Order, e.Slug)
		if e.Err != nil {
			snap.Errs[e.Slug] = e.Err
			continue
		}
		if e.Status != nil {
			snap.Status[e.Slug] = *e.Status
		}
	}
	sort.Strings(snap.Order)
	return snap
}

// Load answers the question `LoadFeatureStatus` answers, from the
// capture. A slug that was absent at capture time returns an
// os.ErrNotExist-wrapped error so existing `os.IsNotExist` branches keep
// behaving identically.
func (fs *FeatureSnapshot) Load(slug string) (FeatureStatus, error) {
	if fs == nil {
		return FeatureStatus{}, fmt.Errorf("feature snapshot: not captured")
	}
	if err, bad := fs.Errs[slug]; bad {
		return FeatureStatus{}, err
	}
	st, ok := fs.Status[slug]
	if !ok {
		return FeatureStatus{}, fmt.Errorf("feature %q: %w", slug, os.ErrNotExist)
	}
	return st, nil
}

// List returns every readable status in slug-sorted order — the
// snapshot equivalent of ListFeatures().
func (fs *FeatureSnapshot) List() []FeatureStatus {
	if fs == nil {
		return nil
	}
	out := make([]FeatureStatus, 0, len(fs.Order))
	for _, slug := range fs.Order {
		if st, ok := fs.Status[slug]; ok {
			out = append(out, st)
		}
	}
	return out
}

// Unreadable returns the slugs whose status could not be read, in
// slug-sorted order.
func (fs *FeatureSnapshot) Unreadable() []string {
	if fs == nil {
		return nil
	}
	var out []string
	for _, slug := range fs.Order {
		if _, bad := fs.Errs[slug]; bad {
			out = append(out, slug)
		}
	}
	return out
}

// SetStatus replaces one captured status in place. The ONLY legitimate
// caller is the writer that just persisted that exact value (verify's
// freshness record), so the capture keeps describing the store without
// a re-read.
func (fs *FeatureSnapshot) SetStatus(slug string, st FeatureStatus) {
	if fs == nil || fs.Status == nil {
		return
	}
	if _, bad := fs.Errs[slug]; bad {
		return
	}
	fs.Status[slug] = st
}
