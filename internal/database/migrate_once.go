package database

import (
    "log"
    "sync"
)

// MigrateOnce runs a migration until it succeeds (BUG-84). Unlike sync.Once it
// is only marked done after success, so a transient DB error on the first call
// does not leave the tables missing for the lifetime of the process.
type MigrateOnce struct {
    mu   sync.Mutex
    done bool
}

// Run calls f until it returns nil, then never again.
func (m *MigrateOnce) Run(f func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.done {
        return
    }
    if err := f(); err != nil {
        return
    }
    m.done = true
}

// Migrate runs AutoMigrate for the given models until it succeeds.
func (m *MigrateOnce) Migrate(dst ...interface{}) {
    m.Run(func() error {
        if err := DB.AutoMigrate(dst...); err != nil {
            log.Printf("automigrate failed, will retry on next call: %v", err)
            return err
        }
        return nil
    })
}