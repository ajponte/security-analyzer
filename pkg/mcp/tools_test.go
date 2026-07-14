package mcp

import (
	"path/filepath"
	"testing"
)

func TestIsSafePath(t *testing.T) {
	workspace, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to resolve absolute path of current dir: %v", err)
	}

	tests := []struct {
		name       string
		target     string
		expectSafe bool
	}{
		{
			name:       "direct match",
			target:     workspace,
			expectSafe: true,
		},
		{
			name:       "direct match relative dot",
			target:     ".",
			expectSafe: true,
		},
		{
			name:       "sub-directory",
			target:     filepath.Join(workspace, "pkg"),
			expectSafe: true,
		},
		{
			name:       "sub-directory relative",
			target:     "pkg/mcp",
			expectSafe: true,
		},
		{
			name:       "parent escaping relative",
			target:     "../",
			expectSafe: false,
		},
		{
			name:       "sibling folder absolute",
			target:     filepath.Join(filepath.Dir(workspace), "other-folder"),
			expectSafe: false,
		},
		{
			name:       "system path escape",
			target:     "/etc/passwd",
			expectSafe: false,
		},
		{
			name:       "deep parent escape",
			target:     "../../../../etc/passwd",
			expectSafe: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			safe, err := isSafePath(workspace, tc.target)
			if err != nil {
				t.Logf("got error: %v", err)
			}
			if safe != tc.expectSafe {
				t.Errorf("expected isSafePath(%q, %q) to return %t, got %t (err: %v)", workspace, tc.target, tc.expectSafe, safe, err)
			}
		})
	}
}
