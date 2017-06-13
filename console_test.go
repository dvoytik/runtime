// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"testing"
)

func TestConsoleFromFile(t *testing.T) {
	console := ConsoleFromFile(os.Stdout)

	if console.File() == nil {
		t.Fatalf("console file is nil")
	}
}

func TestNewConsole(t *testing.T) {
	console, err := newConsole()
	if err != nil {
		t.Fatalf("failed to create a new console: %s", err)
	}
	defer console.Close()

	if console.Path() == "" {
		t.Fatalf("console path is empty")
	}

	if console.File() == nil {
		t.Fatalf("console file is nil")
	}
}

func TestIsTerminal(t *testing.T) {
	var fd uintptr = 4
	if isTerminal(fd) {
		t.Fatalf("Fd %d is not a terminal", fd)
	}

	console, err := newConsole()
	if err != nil {
		t.Fatalf("failed to create a new console: %s", err)
	}
	defer console.Close()

	fd = console.File().Fd()
	if !isTerminal(fd) {
		t.Fatalf("Fd %d is a terminal", fd)
	}
}
