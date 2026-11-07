package netdisco_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Keep the evidence independent of implementation route tables. Updating one
// snapshot or its manifest alone must not silently redefine covered operations.
func TestUpstreamSnapshotIntegrity(t *testing.T) {
	t.Parallel()
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile("testdata/upstream/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var provenance struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		SHA256  string `json:"swagger_sha256"`
	}
	if err := json.Unmarshal(read("provenance.json"), &provenance); err != nil {
		t.Fatal(err)
	}
	snapshot := read("swagger-2.117002.json")
	if got := fmt.Sprintf("%x", sha256.Sum256(snapshot)); got != provenance.SHA256 {
		t.Fatalf("Swagger checksum = %s, want %s", got, provenance.SHA256)
	}
	if provenance.Version != "2.117002" || provenance.Commit != "29324b9b89be895d12cb2614143fc14c4b61ae88" {
		t.Fatal("upstream baseline changed without an explicit contract update")
	}
	var swagger struct {
		Info  struct{ Version string }
		Paths map[string]map[string]struct{ Tags []string }
	}
	if err := json.Unmarshal(snapshot, &swagger); err != nil {
		t.Fatal(err)
	}
	if swagger.Info.Version != provenance.Version {
		t.Fatal("snapshot and provenance versions disagree")
	}
	derived := make(map[string]string)
	for path, methods := range swagger.Paths {
		for method, operation := range methods {
			switch method {
			case "get", "put", "post", "delete", "patch", "head", "options":
				if len(operation.Tags) != 1 {
					t.Fatalf("unexpected category tags for %s %s", method, path)
				}
				derived[strings.ToUpper(method)+" "+path] = operation.Tags[0]
			}
		}
	}
	var operations []struct{ Method, Path, Category string }
	if err := json.Unmarshal(read("operations-2.117002.json"), &operations); err != nil {
		t.Fatal(err)
	}
	manifest := make(map[string]string)
	for _, operation := range operations {
		key := operation.Method + " " + operation.Path
		if _, exists := manifest[key]; exists {
			t.Fatalf("duplicate operation %s", key)
		}
		manifest[key] = operation.Category
	}
	if len(derived) != 79 || !reflect.DeepEqual(derived, manifest) {
		t.Fatalf("manifest does not match all 79 upstream operations: snapshot=%d manifest=%d", len(derived), len(manifest))
	}
}
