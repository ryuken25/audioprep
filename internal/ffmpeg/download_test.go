package ffmpeg

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestExtractBinaries builds a fake BtbN-style zip in memory and checks that
// only ffmpeg.exe / ffprobe.exe come out, from any nested folder.
func TestExtractBinaries(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "fake.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	files := map[string]string{
		"ffmpeg-master-latest-win64-gpl/bin/ffmpeg.exe":  "FFMPEG",
		"ffmpeg-master-latest-win64-gpl/bin/ffprobe.exe": "FFPROBE",
		"ffmpeg-master-latest-win64-gpl/bin/ffplay.exe":  "FFPLAY",
		"ffmpeg-master-latest-win64-gpl/LICENSE.txt":     "GPL",
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()

	dest := filepath.Join(dir, "bin")
	if err := extractBinaries(zipPath, dest); err != nil {
		t.Fatalf("extractBinaries: %v", err)
	}
	for name, want := range map[string]string{"ffmpeg.exe": "FFMPEG", "ffprobe.exe": "FFPROBE"} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "ffplay.exe")); err == nil {
		t.Error("ffplay.exe should not have been extracted")
	}
	if _, err := os.Stat(filepath.Join(dest, "ffmpeg.exe.part")); err == nil {
		t.Error("temp .part file left behind")
	}
}

func TestExtractBinaries_NotAZip(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.zip")
	os.WriteFile(bad, []byte("<html>rate limited</html>"), 0o644)
	if err := extractBinaries(bad, dir); err == nil {
		t.Error("expected an error for a non-zip file")
	}
}

func TestExtractBinaries_MissingBinary(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "partial.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("bin/ffmpeg.exe")
	w.Write([]byte("x"))
	zw.Close()
	f.Close()
	if err := extractBinaries(zipPath, filepath.Join(dir, "out")); err == nil {
		t.Error("expected an error when ffprobe.exe is missing")
	}
}

// TestDownload_Real hits GitHub and pulls the actual ~150 MB build. It only
// runs when AUDIOPREP_TEST_DOWNLOAD=1 so normal test runs stay offline and
// fast. This is the "first-run download works" check from the spec.
func TestDownload_Real(t *testing.T) {
	if os.Getenv("AUDIOPREP_TEST_DOWNLOAD") != "1" {
		t.Skip("set AUDIOPREP_TEST_DOWNLOAD=1 to run the real download")
	}
	dir := t.TempDir()
	var last DownloadProgress
	var reports int
	err := Download(context.Background(), dir, func(p DownloadProgress) {
		reports++
		last = p
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if reports < 10 {
		t.Errorf("only %d progress reports for a 100+ MB file", reports)
	}
	if last.Stage != "extracting" {
		t.Errorf("last stage = %q, want extracting", last.Stage)
	}
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s missing after download: %v", name, err)
		}
		if st.Size() < 10*1024*1024 {
			t.Errorf("%s is only %d bytes; not a real binary", name, st.Size())
		}
	}
	// No leftover zip or .part files.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "ffmpeg.exe" && e.Name() != "ffprobe.exe" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
	// And the downloaded ffmpeg actually runs.
	ver, err := Version(context.Background(), filepath.Join(dir, "ffmpeg.exe"))
	if err != nil {
		t.Fatalf("downloaded ffmpeg does not run: %v", err)
	}
	t.Logf("downloaded: %s", ver)
}
