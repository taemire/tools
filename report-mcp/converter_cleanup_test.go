package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestConverterCleanupIsJobLocal(t *testing.T) {
	root := t.TempDir()
	job := filepath.Join(root, "job-a")
	other := filepath.Join(root, "job-b")
	outside := filepath.Join(root, "outside")
	for _, dir := range []string{filepath.Join(job, ".md2pdf-a"), filepath.Join(other, ".md2pdf-b"), filepath.Join(job, "source"), outside} {
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(dir, "keep"), []byte("synthetic"), 0600)
	}
	final := filepath.Join(job, "report.pdf")
	os.WriteFile(final, []byte("published report"), 0600)
	if runtime.GOOS != "windows" {
		if e := os.Symlink(outside, filepath.Join(job, ".md2pdf-link")); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 2; i++ {
		if e := cleanupConverterWorkspaces(job); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := os.Lstat(filepath.Join(job, ".md2pdf-a")); !os.IsNotExist(e) {
		t.Fatal("owned workspace retained")
	}
	for _, p := range []string{final, filepath.Join(job, "source/keep"), filepath.Join(other, ".md2pdf-b/keep"), filepath.Join(outside, "keep")} {
		if _, e := os.Stat(p); e != nil {
			t.Fatal("removed unrelated data", p, e)
		}
	}
}

func TestConverterTimeoutCleansIntermediateHTML(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper")
	}
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	out := filepath.Join(root, "report-pdf")
	os.MkdirAll(filepath.Join(sessions, "task-1"), 0700)
	os.MkdirAll(out, 0700)
	os.WriteFile(filepath.Join(sessions, "task-1/input.md"), []byte("## Synthetic timeout\n\nBody."), 0600)
	converter := filepath.Join(root, "converter.sh")
	script := `#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
 if [ "$1" = '-o' ]; then out="$2"; shift 2; else shift; fi
done
dir=$(dirname "$out")
mkdir -m 700 "$dir/.md2pdf-inflight"
printf '%s' 'intermediate HTML' > "$dir/.md2pdf-inflight/pass1.html"
printf '%s' 'started' > "$dir/started"
exec sleep 10
`
	if e := os.WriteFile(converter, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	a, e := newApp(config{sessions, out, out, converter})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, e := a.render(ctx, renderInput{SessionID: "task-1", MarkdownFile: "input.md"})
	if e == nil || ctx.Err() != context.DeadlineExceeded || r.Status != "FAILED" {
		t.Fatal("did not exercise forced timeout", r.Status, e, ctx.Err())
	}
	job := filepath.Join(out, "task-1", r.JobID)
	if _, e := os.Stat(filepath.Join(job, "started")); e != nil {
		t.Fatal("converter did not reach its intermediate write", e)
	}
	if _, e := os.Stat(filepath.Join(job, ".md2pdf-inflight")); !os.IsNotExist(e) {
		t.Fatal("timeout left intermediate HTML", e)
	}
}
