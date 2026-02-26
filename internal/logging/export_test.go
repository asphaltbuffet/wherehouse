package logging

import "sync"

// resetForTesting resets all package-level state so Init() can be called
// again in tests. Must only be called from test files (_test.go).
func resetForTesting() {
	mu.Lock()
	if file != nil {
		_ = file.Close()
		file = nil
	}
	active = nil
	errInit = nil
	mu.Unlock()
	once = sync.Once{}
}
