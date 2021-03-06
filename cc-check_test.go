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
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createFile(file, contents string) error {
	return ioutil.WriteFile(file, []byte(contents), testFileMode)
}

func TestCheckGetCPUInfo(t *testing.T) {
	type testData struct {
		contents       string
		expectedResult string
	}

	data := []testData{
		{"", ""},
		{" ", " "},
		{"\n", "\n"},
		{"\n\n", "\n\n"},
		{"hello\n", "hello\n"},
		{"hello\n\n", "hello\n\n"},
		{"hello\n\nworld\n\n", "hello\n\n"},
		{"foo\n\nbar\nbaz\n\n", "foo\n\n"},
	}

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "cpuinfo")
	// file doesn't exist
	_, err = getCPUInfo(file)
	assert.Error(t, err)

	for _, d := range data {
		err = ioutil.WriteFile(file, []byte(d.contents), testFileMode)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file)

		contents, err := getCPUInfo(file)
		assert.NoError(t, err, "expected no error")

		assert.Equal(t, d.expectedResult, contents)
	}
}

func TestCheckFindAnchoredString(t *testing.T) {
	type testData struct {
		haystack      string
		needle        string
		expectSuccess bool
	}

	data := []testData{
		{"", "", false},
		{"", "foo", false},
		{"foo", "", false},
		{"food", "foo", false},
		{"foo", "foo", true},
		{"foo bar", "foo", true},
		{"foo bar baz", "bar", true},
	}

	for _, d := range data {
		result := findAnchoredString(d.haystack, d.needle)

		if d.expectSuccess {
			assert.True(t, result)
		} else {
			assert.False(t, result)
		}
	}
}

func TestCheckGetCPUFlags(t *testing.T) {
	type testData struct {
		cpuinfo       string
		expectedFlags string
	}

	data := []testData{
		{"", ""},
		{"foo", ""},
		{"foo bar", ""},
		{":", ""},
		{"flags", ""},
		{"flags:", ""},
		{"flags: a b c", "a b c"},
		{"flags: a b c foo bar d", "a b c foo bar d"},
	}

	for _, d := range data {
		result := getCPUFlags(d.cpuinfo)
		assert.Equal(t, d.expectedFlags, result)
	}
}

func TestCheckCheckCPUFlags(t *testing.T) {
	type testData struct {
		cpuflags    string
		required    map[string]string
		expectError bool
	}

	data := []testData{
		{
			"",
			map[string]string{},
			true,
		},
		{
			"",
			map[string]string{
				"a": "A flag",
			},
			true,
		},
		{
			"",
			map[string]string{
				"a": "A flag",
				"b": "B flag",
			},
			true,
		},
		{
			"a b c",
			map[string]string{
				"b": "B flag",
			},
			false,
		},
	}

	for _, d := range data {
		err := checkCPUFlags(d.cpuflags, d.required)
		if d.expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestCheckCheckCPUAttribs(t *testing.T) {
	type testData struct {
		cpuinfo     string
		required    map[string]string
		expectError bool
	}

	data := []testData{
		{
			"",
			map[string]string{},
			true,
		},
		{
			"",
			map[string]string{
				"a": "",
			},
			true,
		},
		{
			"a: b",
			map[string]string{
				"b": "B attribute",
			},
			false,
		},
		{
			"a: b\nc: d\ne: f",
			map[string]string{
				"b": "B attribute",
			},
			false,
		},
		{
			"a: b\n",
			map[string]string{
				"b": "B attribute",
				"c": "C attribute",
				"d": "D attribute",
			},
			true,
		},
		{
			"a: b\nc: d\ne: f",
			map[string]string{
				"b": "B attribute",
				"d": "D attribute",
				"f": "F attribute",
			},
			false,
		},
	}

	for _, d := range data {
		err := checkCPUAttribs(d.cpuinfo, d.required)
		if d.expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestCheckHaveKernelModule(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	savedModInfoCmd := modInfoCmd
	savedSysModuleDir := sysModuleDir

	// XXX: override (fake the modprobe command failing)
	modInfoCmd = "false"
	sysModuleDir = filepath.Join(dir, "sys/module")

	defer func() {
		modInfoCmd = savedModInfoCmd
		sysModuleDir = savedSysModuleDir
	}()

	err = os.MkdirAll(sysModuleDir, testDirMode)
	if err != nil {
		t.Fatal(err)
	}

	module := "foo"

	result := haveKernelModule(module)
	assert.False(t, result)

	// XXX: override - make our fake "modprobe" succeed
	modInfoCmd = "true"

	result = haveKernelModule(module)
	assert.True(t, result)

	// disable "modprobe" again
	modInfoCmd = "false"

	fooDir := filepath.Join(sysModuleDir, module)
	err = os.MkdirAll(fooDir, testDirMode)
	if err != nil {
		t.Fatal(err)
	}

	result = haveKernelModule(module)
	assert.True(t, result)
}

func TestCheckCheckKernelModules(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	savedModInfoCmd := modInfoCmd
	savedSysModuleDir := sysModuleDir

	// XXX: override (fake the modprobe command failing)
	modInfoCmd = "false"
	sysModuleDir = filepath.Join(dir, "sys/module")

	defer func() {
		modInfoCmd = savedModInfoCmd
		sysModuleDir = savedSysModuleDir
	}()

	err = os.MkdirAll(sysModuleDir, testDirMode)
	if err != nil {
		t.Fatal(err)
	}

	testData := map[string]kernelModule{
		"foo": {
			desc:       "desc",
			parameters: map[string]string{},
		},
		"bar": {
			desc: "desc",
			parameters: map[string]string{
				"param1": "hello",
				"param2": "world",
				"param3": "a",
				"param4": ".",
			},
		},
	}

	err = checkKernelModules(map[string]kernelModule{})
	// No required modules means no error
	assert.NoError(t, err)

	err = checkKernelModules(testData)
	// No modules exist
	assert.Error(t, err)

	for module, details := range testData {
		path := filepath.Join(sysModuleDir, module)
		err = os.MkdirAll(path, testDirMode)
		if err != nil {
			t.Fatal(err)
		}

		paramDir := filepath.Join(path, "parameters")
		err = os.MkdirAll(paramDir, testDirMode)
		if err != nil {
			t.Fatal(err)
		}

		for param, value := range details.parameters {
			paramPath := filepath.Join(paramDir, param)
			err = createFile(paramPath, value)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	err = checkKernelModules(testData)
	assert.NoError(t, err)
}

func TestCheckHostIsClearContainersCapable(t *testing.T) {
	type testModuleData struct {
		path     string
		isDir    bool
		contents string
	}

	type testCPUData struct {
		vendorID    string
		flags       string
		expectError bool
	}

	cpuData := []testCPUData{
		{"", "", true},
		{"Intel", "", true},
		{"GenuineIntel", "", true},
		{"GenuineIntel", "lm", true},
		{"GenuineIntel", "lm vmx", true},
		{"GenuineIntel", "lm vmx sse4_1", false},
	}

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "cpuinfo")

	savedSysModuleDir := sysModuleDir

	// XXX: override
	sysModuleDir = filepath.Join(dir, "sys/module")

	defer func() {
		sysModuleDir = savedSysModuleDir
	}()

	err = os.MkdirAll(sysModuleDir, testDirMode)
	if err != nil {
		t.Fatal(err)
	}

	moduleData := []testModuleData{
		{filepath.Join(sysModuleDir, "kvm"), true, ""},
		{filepath.Join(sysModuleDir, "kvm_intel"), true, ""},
		{filepath.Join(sysModuleDir, "kvm_intel/parameters/nested"), false, "Y"},
		{filepath.Join(sysModuleDir, "kvm_intel/parameters/unrestricted_guest"), false, "Y"},
	}

	for _, d := range moduleData {
		var dir string

		if d.isDir {
			dir = d.path
		} else {
			dir = path.Dir(d.path)
		}

		err = os.MkdirAll(dir, testDirMode)
		if err != nil {
			t.Fatal(err)
		}

		if !d.isDir {
			err = createFile(d.path, d.contents)
			if err != nil {
				t.Fatal(err)
			}
		}

		err = hostIsClearContainersCapable(file)
		// file doesn't exist
		assert.Error(t, err)
	}

	// all the modules file have now been created, so deal with the
	// cpuinfo data.

	for _, d := range cpuData {
		err = makeCPUInfoFile(file, d.vendorID, d.flags)
		assert.NoError(t, err)

		err = hostIsClearContainersCapable(file)
		if d.expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
