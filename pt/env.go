package pt

import (
	"os"
	"sync"
)

var envLock sync.Mutex

// TestEnv is a utility for setting and resetting environment
// variables for tests. An empty test env is ready for use and will
// acquire a lock on environment changes at the first environment
// change, and releases it when Cleanup() is called.
type TestEnv struct {
	initOnce    sync.Once
	cleanupOnce sync.Once

	replaced map[string]string
	unset    map[string]bool
}

func (te *TestEnv) init() {
	te.initOnce.Do(func() {
		te.replaced = make(map[string]string)
		te.unset = make(map[string]bool)
		envLock.Lock()
	})
}

// SetAll the provided environment variables. Acquires an environment
// lock when first called. Panics if called after Cleanup().
func (te *TestEnv) SetAll(vars map[string]string) {
	for k, v := range vars {
		te.Set(k, v)
	}
}

// Set a single environment variable. Acquires an environment lock
// when first called. Panics if called after Cleanup().
func (te *TestEnv) Set(name, value string) {
	te.init()

	_, replaced := te.replaced[name]
	if !replaced && !te.unset[name] {
		envVal, exists := os.LookupEnv(name)
		if exists {
			te.replaced[name] = envVal
		} else {
			te.unset[name] = true
		}
	}
	os.Setenv(name, value)
}

// Cleanup restores the environment to its original state and releases
// the environment lock. Safe to call multiple times.
func (te *TestEnv) Cleanup() {
	te.cleanupOnce.Do(func() {
		for k, unset := range te.unset {
			if unset {
				os.Unsetenv(k)
			}
		}

		te.unset = nil

		for k, v := range te.replaced {
			os.Setenv(k, v)
		}

		te.replaced = nil

		envLock.Unlock()
	})
}
