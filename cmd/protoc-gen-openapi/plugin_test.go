// Copyright 2020 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/diff"
)

const testPlugin = `protoc-gen-openapi-test`

var (
	regenerate bool
	protoc     string
	pluginPath string
)

func TestGenOpenAPI(t *testing.T) {
	fixtureTest(t, "library example", "examples/google/example/library/v1/library.proto")
	fixtureTest(t, "additional bindings", "examples/tests/additional_bindings/message.proto")
	fixtureTest(t, "allof wrapping", "examples/tests/allofwrap/message.proto")
	fixtureTest(t, "body mapping", "examples/tests/bodymapping/message.proto")
	fixtureTest(t, "linter comments", "examples/tests/lintercomments/message.proto")
	fixtureTest(t, "map fields", "examples/tests/mapfields/message.proto")
	fixtureTest(t, "skip unannotated services", "examples/tests/noannotations/message.proto")
	fixtureTest(t, "openapiv3annotations", "examples/tests/openapiv3annotations/message.proto")
	fixtureTest(t, "path parameters", "examples/tests/pathparams/message.proto")
	fixtureTest(t, "path param hints", "examples/tests/pathparamhints/message.proto")
	fixtureTest(t, "protobuf types", "examples/tests/protobuftypes/message.proto")
	fixtureTest(t, "json options", "examples/tests/jsonoptions/message.proto")
	optionFixtureTest(t, "proto naming", "examples/tests/naming_proto/message.proto", "naming=proto")
	optionFixtureTest(t, "string enums", "examples/tests/enumoptions/message.proto", "enum_type=string")
	optionFixtureTest(t, "wildcard_body_dedup", "examples/tests/wildcard_body_dedup/message.proto", "wildcard_body_dedup=true")
	optionFixtureTest(t, "no_default_response", "examples/tests/no_default_response/message.proto", "default_response=false")
	optionFixtureTest(t, "circular depth", "examples/tests/circulardepth/message.proto", "depth=3")
	optionFixtureTest(t, "fully-qualified schema naming", "examples/tests/fq_schema_naming/message.proto", "fq_schema_naming=true")
}

func TestOutputMode(t *testing.T) {
	fixtureDir := "examples/tests/output_mode/source_relative"
	protoFiles := []string{
		fixtureDir + "/service_a/testservice.proto",
		fixtureDir + "/service_b/testservice.proto",
	}

	t.Run("source_relative", func(t *testing.T) {
		outputDir, err := generateOpenAPI(t, protoFiles, "output_mode=source_relative")
		if err != nil {
			t.Fatalf("generating openapi: %v", err)
		}
		outputDir = filepath.Join(outputDir, "tests/output_mode/source_relative")

		if regenerate {
			if diffTest(outputDir, fixtureDir) == nil {
				t.Skip("no change to fixtures")
			}
			if err := cpr(outputDir, fixtureDir); err != nil {
				t.Fatalf("error copying regenerated fixtures: %v", err)
			}
			t.Log("regenerated fixtures")
			return
		}
		if err := diffTest(outputDir, fixtureDir); err != nil {
			t.Fatalf("comparing fixtures:\n%v", err)
		}
	})

	t.Run("merged", func(t *testing.T) {
		// just compared against (or rewrite) the merged proto
		fixtureDir := "examples/tests/output_mode/merged"
		outputDir, err := generateOpenAPI(t, protoFiles)
		if err != nil {
			t.Fatalf("generating openapi: %v", err)
		}
		if regenerate {
			if diffTest(outputDir, fixtureDir) == nil {
				t.Skip("no change to fixtures")
			}
			if err := cpr(outputDir, fixtureDir); err != nil {
				t.Fatalf("error copying regenerated fixtures: %v", err)
			}
			t.Log("regenerated fixtures")
			return
		}
		if err := diffTest(outputDir, fixtureDir); err != nil {
			t.Fatalf("comparing fixtures:\n%v", err)
		}
	})
}

func TestMain(m *testing.M) {
	var err error
	protoc, err = exec.LookPath("protoc")
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc is required for fixture tests: %v", err)
		os.Exit(1)
	}
	pluginTmp, err := os.MkdirTemp("", testPlugin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MkdirTemp: %v", err)
		os.Exit(1)
	}
	defer os.RemoveAll(pluginTmp)

	pluginPath = filepath.Join(pluginTmp, testPlugin)
	cmd := exec.Command("go", "build", "-o", pluginPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(pluginTmp)
		fmt.Fprintf(os.Stderr, "failed to build plugin: %v", err)
		os.Exit(1)
	}

	regenerate = strings.ToLower(os.Getenv("GNOSTIC_REGEN_FIXTURES")) == "true"
	exitCode := m.Run()
	if exitCode == 0 && regenerate {
		fmt.Fprint(os.Stderr, "fixtures have been regenerated, you may now run tests")
		os.Exit(1)
	}
	os.Exit(exitCode)
}

// optionFixtureTest verifies that the generated code from protoFile matches the
// openapi.yaml fixture in the same directory. It will also verify that the
// output is changed from the default settings and fail the test if they are the
// same.
func optionFixtureTest(t *testing.T, testName string, protoFile string, pluginArg string) {
	t.Helper()
	t.Run(testName, func(t *testing.T) {
		t.Helper()
		fixtureDir := filepath.Dir(protoFile)
		protoFiles := []string{protoFile}
		outputDir, err := generateOpenAPI(t, protoFiles, pluginArg)
		if err != nil {
			t.Fatalf("generating openapi: %v", err)
		}
		defaultOutputDir, err := generateOpenAPI(t, protoFiles)
		if err != nil {
			t.Fatalf("generating default output: %v", err)
		}
		if err := diffTest(defaultOutputDir, outputDir); err == nil {
			t.Fatalf("output was identical to default output")
		}
		if regenerate {
			if diffTest(outputDir, fixtureDir) == nil {
				t.Skip("no change to fixtures")
			}
			if err := cpr(outputDir, fixtureDir); err != nil {
				t.Fatalf("error copying regenerated fixtures")
			}
			t.Log("regenerated fixtures")
			return
		}
		if err := diffTest(outputDir, fixtureDir); err != nil {
			t.Fatalf("comparing fixtures: \n%v", err)
		}
	})
}

// fixtureTest verifies that the generated code from protoFile matches the
// openapi.yaml fixture in the same directory.
func fixtureTest(t *testing.T, testName string, protoFile string) {
	t.Helper()
	t.Run(testName, func(t *testing.T) {
		t.Helper()
		fixtureDir := filepath.Dir(protoFile)
		protoFiles := []string{protoFile}
		outputDir, err := generateOpenAPI(t, protoFiles)
		if err != nil {
			t.Fatalf("generating openapi: %v", err)
		}
		if regenerate {
			if diffTest(outputDir, fixtureDir) == nil {
				t.Skip("no change to fixtures")
			}
			if err := cpr(outputDir, fixtureDir); err != nil {
				t.Fatalf("error copying regenerated fixtures")
			}
			t.Log("regenerated fixtures")
			return
		}
		if err := diffTest(outputDir, fixtureDir); err != nil {
			t.Fatalf("output did not match fixture data\n%v", err)
		}
	})
}

func generateOpenAPI(t *testing.T, protoFiles []string, pluginArgs ...string) (string, error) {
	t.Helper()
	outputDir := t.TempDir()
	protocArgs := protocArgs(protoFiles, outputDir)
	for _, arg := range pluginArgs {
		protocArgs = append(protocArgs, "--openapi_opt="+arg)
	}

	cmdOut, err := exec.Command(protoc, protocArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("protoc invocation failed: %v\n%s", err, cmdOut)
	}
	return outputDir, nil
}

func protocArgs(protoFiles []string, outputDir string) []string {
	args := append([]string{
		"-I", "../../",
		"-I", "../../third_party",
		"-I", "examples",
		"--plugin", "protoc-gen-openapi=" + pluginPath,
	}, protoFiles...)
	return append(args,
		"--openapi_out="+outputDir,
	)
}

// diffTest compares every file under gotDir with the corresponding file of the
// path under wantDir, returning an error along with the diff output if any file
// differs.
//
// files which only exist in dirB will be skipped
func diffTest(gotDir, wantDir string) error {
	return filepath.WalkDir(gotDir, func(gotFile string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(gotDir, gotFile)
		wantFile := filepath.Join(wantDir, rel)
		got, err := os.ReadFile(gotFile)
		if err != nil {
			return fmt.Errorf("read %s: %v", gotFile, err)
		}
		want, err := os.ReadFile(wantFile)
		if err != nil {
			return fmt.Errorf("read %s: %v", wantFile, err)
		}
		if bytes.Equal(got, want) {
			return nil
		}
		buf := new(bytes.Buffer)
		diff.Text(wantFile, filepath.Join("<test output>", rel), want, got, buf)
		return fmt.Errorf("%s differs:\n%s", rel, buf.String())
	})
}

func cpr(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
