package sqlitesnapshot

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDigestDetectsSameSizeSourceChange(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "copy")
	original := []byte("original")
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := copyFile(context.Background(), source, dest, info)
	if err != nil || digest != sha256.Sum256(original) {
		t.Fatalf("private copy digest: %x, %v", digest, err)
	}
	if err := os.WriteFile(source, []byte("modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(source)
	if err != nil || !sameState(info, after) {
		t.Fatalf("fixture metadata must match: %v", err)
	}
	verification, err := readFile(context.Background(), source, info, io.Discard)
	if err != nil || verification == digest {
		t.Fatalf("verification missed source change: %v", err)
	}
}

func TestCopyFileRefusalsPreserveFiles(t *testing.T) {
	for _, tc := range []struct {
		name, sourceText, destText string
		cancelled                  bool
		wantError                  error
	}{
		{"cancelled", "synthetic", "", true, context.Canceled},
		{"existing destination", "replaced", "keep existing", false, os.ErrExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source, dest := filepath.Join(root, "source"), filepath.Join(root, "copy")
			if err := os.WriteFile(source, []byte(tc.sourceText), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.destText != "" {
				if err := os.WriteFile(dest, []byte(tc.destText), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			info, err := os.Stat(source)
			if err != nil {
				t.Fatal(err)
			}
			var destBefore os.FileInfo
			if tc.destText != "" {
				destBefore, err = os.Stat(dest)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			if _, err := copyFile(ctx, source, dest, info); !errors.Is(err, tc.wantError) {
				t.Fatalf("copy refusal = %v, want %v", err, tc.wantError)
			}
			got, err := os.ReadFile(source)
			if err != nil || string(got) != tc.sourceText {
				t.Fatalf("refused copy changed source: %q, %v", got, err)
			}
			after, err := os.Stat(source)
			if err != nil || !sameState(info, after) {
				t.Fatalf("refused copy changed source metadata: %v", err)
			}
			if tc.destText == "" {
				if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("destination created after refusal: %v", err)
				}
				return
			}
			got, err = os.ReadFile(dest)
			if err != nil || string(got) != tc.destText {
				t.Fatalf("existing destination changed: %q, %v", got, err)
			}
			destInfo, err := os.Stat(dest)
			if err != nil || !sameState(destBefore, destInfo) {
				t.Fatalf("existing destination metadata changed: %v, %v", destInfo, err)
			}
		})
	}
}
