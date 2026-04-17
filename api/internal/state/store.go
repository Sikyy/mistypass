package state

// Store saves and loads module snapshots as JSON payloads.
type Store interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}
