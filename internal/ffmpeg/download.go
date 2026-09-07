package ffmpeg

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadURL is the BtbN nightly. The "latest" tag is a rolling release with
// a stable asset name, so this URL never needs updating.
const DownloadURL = "https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-win64-gpl.zip"

// DownloadProgress reports bytes so far and the total (0 when the server did
// not send Content-Length).
type DownloadProgress struct {
	Done  int64
	Total int64
	Stage string // "downloading" or "extracting"
}

// Download fetches the BtbN ffmpeg build, verifies it is a zip, and extracts
// only ffmpeg.exe and ffprobe.exe into destDir. It reports progress through
// the callback (which may be nil) and honours ctx for cancellation.
//
// The zip is streamed to a temp file next to destDir rather than held in
// memory: it is ~170 MB and there is no reason to allocate that.
func Download(ctx context.Context, destDir string, report func(DownloadProgress)) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	tmp, err := os.CreateTemp(destDir, "ffmpeg-*.zip.part")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Always clean up the partial download; on success the extracted exes are
	// what we keep, not the zip.
	defer os.Remove(tmpPath)

	if err := fetch(ctx, DownloadURL, tmp, report); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if report != nil {
		report(DownloadProgress{Stage: "extracting"})
	}
	return extractBinaries(tmpPath, destDir)
}

func fetch(ctx context.Context, url string, w io.Writer, report func(DownloadProgress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "audioprep/1.0 (+https://github.com/ryuken25/audioprep)")

	resp, err := newDownloadClient().Do(req)
	if err != nil {
		return fmt.Errorf("download ffmpeg: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download ffmpeg: server returned %s", resp.Status)
	}

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256*1024)
	lastReport := time.Now()

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return fmt.Errorf("write download: %w", werr)
			}
			done += int64(n)
			// Throttle UI updates to ~10/s; the progress bar cannot show more.
			if report != nil && time.Since(lastReport) > 100*time.Millisecond {
				report(DownloadProgress{Done: done, Total: total, Stage: "downloading"})
				lastReport = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("download ffmpeg: %w", rerr)
		}
	}
	if report != nil {
		report(DownloadProgress{Done: done, Total: total, Stage: "downloading"})
	}
	return nil
}

// newDownloadClient returns an HTTP client with HTTP/2 switched off.
//
// Go's HTTP/2 client advertises a small flow-control window, and against
// GitHub's release CDN that throttled this 170 MB download to ~0.3 MB/s on a
// connection that does 6 MB/s over HTTP/1.1 (measured; see DECISIONS.md).
// Setting TLSNextProto to an empty map is the documented way to opt out of
// h2 while keeping everything else (proxy from environment, timeouts) from
// the default transport. No overall client timeout: ctx handles cancel.
func newDownloadClient() *http.Client {
	// A fresh Transport, not a Clone() of the default one: Clone() forces
	// the default transport's h2 setup first, and the copied TLS config then
	// still advertises "h2" via ALPN. The server picks h2, we speak HTTP/1.1,
	// and the connection dies with EOF. Owning the tls.Config avoids that.
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     false,
		TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{Transport: tr}
}

// extractBinaries pulls ffmpeg.exe and ffprobe.exe out of the zip regardless
// of what folder they sit in. zip.OpenReader also validates the archive, which
// doubles as our "is this really a zip" check.
func extractBinaries(zipPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("downloaded file is not a valid zip: %w", err)
	}
	defer zr.Close()

	wanted := map[string]bool{"ffmpeg.exe": false, "ffprobe.exe": false}
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if _, ok := wanted[base]; !ok || f.FileInfo().IsDir() {
			continue
		}
		if err := extractOne(f, filepath.Join(destDir, base)); err != nil {
			return err
		}
		wanted[base] = true
	}
	for name, got := range wanted {
		if !got {
			return fmt.Errorf("%s not found inside the downloaded zip", name)
		}
	}
	return nil
}

func extractOne(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Write to a temp name then rename, so a crash mid-extract never leaves a
	// half-written ffmpeg.exe that Locate() would happily pick up.
	tmp := dest + ".part"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	os.Remove(dest) // ignore error; may not exist
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	return nil
}

// IsNetworkError is a best-effort classifier so the UI can say "check your
// connection" instead of dumping a Go error.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "download ffmpeg") ||
		strings.Contains(s, "dial tcp") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "TLS")
}
