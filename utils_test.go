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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileExists(t *testing.T) {
	dir, err := ioutil.TempDir(testDir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "foo")

	assert.False(t, fileExists(file),
		fmt.Sprintf("File %q should not exist", file))

	err = createEmptyFile(file)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, fileExists(file),
		fmt.Sprintf("File %q should exist", file))
}

func TestGetFileContents(t *testing.T) {
	type testData struct {
		contents string
	}

	data := []testData{
		{""},
		{" "},
		{"\n"},
		{"\n\n"},
		{"\n\n\n"},
		{"foo"},
		{"foo\nbar"},
		{"processor   : 0\nvendor_id   : GenuineIntel\n"},
	}

	dir, err := ioutil.TempDir(testDir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "foo")

	// file doesn't exist
	_, err = getFileContents(file)
	assert.Error(t, err)

	for _, d := range data {
		// create the file
		err = ioutil.WriteFile(file, []byte(d.contents), testFileMode)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(file)

		contents, err := getFileContents(file)
		assert.NoError(t, err)
		assert.Equal(t, contents, d.contents)
	}
}

func TestGetKernelVersion(t *testing.T) {
	type testData struct {
		contents        string
		expectedVersion string
		expectError     bool
	}

	const validVersion = "1.2.3-4.5.x86_64"
	validContents := fmt.Sprintf("Linux version %s blah blah blah ...", validVersion)

	data := []testData{
		{"", "", true},
		{"invalid contents", "", true},
		{validContents, validVersion, false},
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpdir)

	file := filepath.Join(tmpdir, "proc-version")

	// override
	procVersion = file

	_, err = getKernelVersion()
	// ENOENT
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	for _, d := range data {
		err := createFile(file, d.contents)
		assert.NoError(t, err)

		version, err := getKernelVersion()
		if d.expectError {
			assert.Error(t, err, fmt.Sprintf("%+v", d))
			continue
		} else {
			assert.NoError(t, err, fmt.Sprintf("%+v", d))
			assert.Equal(t, d.expectedVersion, version)
		}
	}
}

func TestGetDistroDetails(t *testing.T) {
	type testData struct {
		clrContents     string
		nonClrContents  string
		expectedName    string
		expectedVersion string
		expectError     bool
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpdir)

	testOSRelease := filepath.Join(tmpdir, "os-release")
	testOSReleaseClr := filepath.Join(tmpdir, "os-release-clr")

	const clrExpectedName = "clr"
	const clrExpectedVersion = "1.2.3-4"
	clrContents := fmt.Sprintf(`
HELLO=world
NAME="%s"
FOO=bar
VERSION_ID="%s"
`, clrExpectedName, clrExpectedVersion)

	const nonClrExpectedName = "not-clr"
	const nonClrExpectedVersion = "999"
	nonClrContents := fmt.Sprintf(`
HELLO=world
NAME="%s"
FOO=bar
VERSION_ID="%s"
`, nonClrExpectedName, nonClrExpectedVersion)

	// override
	osRelease = testOSRelease
	osReleaseClr = testOSReleaseClr

	_, _, err = getDistroDetails()
	// ENOENT
	assert.Error(t, err)

	data := []testData{
		{"", "", "", "", true},
		{"invalid", "", "", "", true},
		{clrContents, "", clrExpectedName, clrExpectedVersion, false},
		{"", nonClrContents, nonClrExpectedName, nonClrExpectedVersion, false},
		{clrContents, nonClrContents, nonClrExpectedName, nonClrExpectedVersion, false},
	}

	for _, d := range data {
		err := createFile(osRelease, d.nonClrContents)
		assert.NoError(t, err)

		err = createFile(osReleaseClr, d.clrContents)
		assert.NoError(t, err)

		name, version, err := getDistroDetails()
		if d.expectError {
			assert.Error(t, err, fmt.Sprintf("%+v", d))
			continue
		} else {
			assert.NoError(t, err, fmt.Sprintf("%+v", d))
			assert.Equal(t, d.expectedName, name)
			assert.Equal(t, d.expectedVersion, version)
		}
	}
}

func TestGetCPUDetails(t *testing.T) {
	type testData struct {
		contents       string
		expectedVendor string
		expectedModel  string
		expectError    bool
	}

	const validVendorName = "a vendor"
	validVendor := fmt.Sprintf(`vendor_id	: %s`, validVendorName)

	const validModelName = "some CPU model"
	validModel := fmt.Sprintf(`model name	: %s`, validModelName)

	validContents := fmt.Sprintf(`
a	: b
%s
foo	: bar
%s
`, validVendor, validModel)

	data := []testData{
		{"", "", "", true},
		{"invalid", "", "", true},
		{"vendor_id", "", "", true},
		{validVendor, "", "", true},
		{validModel, "", "", true},
		{validContents, validVendorName, validModelName, false},
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpdir)

	testProcCPUInfo := filepath.Join(tmpdir, "cpuinfo")

	// override
	procCPUInfo = testProcCPUInfo

	_, _, err = getCPUDetails()
	// ENOENT
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	for _, d := range data {
		err := createFile(procCPUInfo, d.contents)
		assert.NoError(t, err)

		vendor, model, err := getCPUDetails()

		if d.expectError {
			assert.Error(t, err, fmt.Sprintf("%+v", d))
			continue
		} else {
			assert.NoError(t, err, fmt.Sprintf("%+v", d))
			assert.Equal(t, d.expectedVendor, vendor)
			assert.Equal(t, d.expectedModel, model)
		}
	}
}
