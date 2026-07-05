package main

import (
	"fmt"
	"os"
	"sync"
)

var (
	fileMu      sync.Mutex
	fileHandles = make(map[string]*os.File)
)

func writeToFile(filename, line string) {
	fileMu.Lock()
	defer fileMu.Unlock()

	f, ok := fileHandles[filename]
	if !ok {
		var err error
		f, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("WARNING: Error reading %s: %v\n", filename, err)
			return
		}
		fileHandles[filename] = f
	}

	fmt.Fprintln(f, line)
}

func closeAllFiles() {
	fileMu.Lock()
	defer fileMu.Unlock()
	for name, f := range fileHandles {
		if err := f.Close(); err != nil {
			fmt.Printf("WARNING: Failed to close %s: %v\n", name, err)
		}
		delete(fileHandles, name)
	}
}


