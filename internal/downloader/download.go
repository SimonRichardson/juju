// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"sync"

	"github.com/juju/errors"
	"gopkg.in/tomb.v2"
)

// Request holds a single download request.
type Request struct {
	// ArchiveSha256 is the string containing the charm archive sha256 hash.
	ArchiveSha256 string

	// URL is the location from which the file will be downloaded.
	URL *url.URL

	// TargetDir is the directory into which the file will be downloaded.
	// It defaults to os.TempDir().
	TargetDir string

	// Verify is used to ensure that the download result is correct. If
	// the download is invalid then the func must return errors.NotValid.
	// If no func is provided then no verification happens.
	Verify func(*os.File) error
}

// StartDownload starts a new download as specified by `req` using
// `openBlob` to actually pull the remote data.
func StartDownload(ctx context.Context, req Request, openBlob func(context.Context, Request) (io.ReadCloser, string, error)) *Download {
	if openBlob == nil {
		openBlob = NewHTTPBlobOpener(false)
	}
	dl := &Download{
		openBlob: openBlob,
	}
	dl.tomb.Go(func() error {
		fileName, hash, err := dl.run(ctx, req)
		if err != nil {
			return errors.Trace(err)
		}

		dl.mutex.Lock()
		dl.fileName = fileName
		dl.hash = hash
		dl.mutex.Unlock()

		return nil
	})
	return dl
}

// Download can download a file from the network.
type Download struct {
	tomb     tomb.Tomb
	mutex    sync.Mutex
	fileName string
	hash     string
	openBlob func(context.Context, Request) (io.ReadCloser, string, error)
}

// Kill aborts the download.
func (dl *Download) Kill() {
	dl.tomb.Kill(nil)
}

// Wait blocks until the download finishes (successfully or
// otherwise), or the download is aborted.
func (dl *Download) Wait() error {
	return dl.tomb.Wait()
}

// FileName returns the name of the file that was downloaded.
func (dl *Download) FileName() string {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()
	return dl.fileName
}

// Hash returns the hash of the file that was downloaded.
func (dl *Download) Hash() string {
	dl.mutex.Lock()
	defer dl.mutex.Unlock()
	return dl.hash
}

func (dl *Download) run(ctx context.Context, req Request) (string, string, error) {
	// TODO(dimitern) 2013-10-03 bug #1234715
	// Add a testing HTTPS storage to verify the
	// disableSSLHostnameVerification behavior here.
	filename, hash, err := dl.download(ctx, req)
	if err != nil {
		return filename, hash, errors.Trace(err)
	} else {
		logger.Infof("download complete (%q)", req.URL)
		err = verifyDownload(filename, req)
		if err != nil {
			_ = os.Remove(filename)
			return "", "", errors.Trace(err)
		}
	}

	return filename, hash, nil
}

func (dl *Download) download(ctx context.Context, req Request) (filename, hash string, err error) {
	logger.Infof("downloading from %s", req.URL)

	dir := req.TargetDir
	if dir == "" {
		dir = os.TempDir()
	}
	tempFile, err := os.CreateTemp(dir, "inprogress-")
	if err != nil {
		return "", "", errors.Trace(err)
	}
	defer func() {
		_ = tempFile.Close()
		if err != nil {
			_ = os.Remove(tempFile.Name())
		}
	}()

	blobReader, hash, err := dl.openBlob(ctx, req)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	defer func() { _ = blobReader.Close() }()

	// We shouldn't embed the context.Context into a struct, but there isn't
	// a better way to do this without changing stdlib.
	reader := &abortableReader{r: blobReader, ctx: ctx}

	if hash == "" {
		logger.Infof("download not verified (%q), no hash supplied", req.URL)
		_, err = io.Copy(tempFile, reader)
		if err != nil {
			return "", "", errors.Trace(err)
		}
		return tempFile.Name(), "", nil
	}

	hasher := sha256.New()
	_, err = io.Copy(tempFile, io.TeeReader(reader, hasher))
	if err != nil {
		return "", "", errors.Trace(err)
	}

	summed := hex.EncodeToString(hasher.Sum(nil))
	if summed != hash {
		return "", "", errors.Errorf("invalid blob returned: expected sha256 %q, got %q", hash, summed)
	}

	return tempFile.Name(), summed, nil
}

// abortableReader wraps a Reader, returning an error from Read calls
// if the abort channel provided is closed.
type abortableReader struct {
	r   io.Reader
	ctx context.Context
}

// Read implements io.Reader.
func (ar *abortableReader) Read(p []byte) (int, error) {
	if err := ar.ctx.Err(); err != nil {
		return 0, errors.Trace(err)
	}
	return ar.r.Read(p)
}

func verifyDownload(filename string, req Request) error {
	if req.Verify == nil {
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return errors.Annotate(err, "opening for verify")
	}
	defer func() { _ = file.Close() }()

	if err := req.Verify(file); err != nil {
		return errors.Trace(err)
	}
	logger.Infof("download verified (%q)", req.URL)
	return nil
}
