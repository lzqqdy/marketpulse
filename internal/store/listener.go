package store

// ChangeListener is notified after store mutations (called under store lock — keep fast).
type ChangeListener func(version uint64)

// AddListener registers a callback for version bumps.
func (s *MarketStore) AddListener(fn ChangeListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *MarketStore) notifyLocked(version uint64) {
	for _, fn := range s.listeners {
		fn(version)
	}
}
