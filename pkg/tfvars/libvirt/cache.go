package libvirt

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/peterbourgon/diskv"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// FIXME: baremetal
// Export cachedImage for reuse by baremetal
func CachedImage(uri string) (string, error) {
	return cachedImage(uri)
}

// cachedImage leaves non-file:// image URIs unalterered.
// Other URIs are retrieved with a local cache at
// $XDG_CACHE_HOME/kni-install/libvirt [1].  This allows you to
// use the same remote image URI multiple times without needing to
// worry about redundant downloads, although you will want to
// periodically blow away your cache.
//
// [1]: https://standards.freedesktop.org/basedir-spec/basedir-spec-0.7.html
func cachedImage(uri string) (string, error) {
	if strings.HasPrefix(uri, "file://") {
		return uri, nil
	}

	logrus.Infof("Fetching OS image: %s", filepath.Base(uri))

	// FIXME: Use os.UserCacheDir() once we bump to Go 1.11
	// baseCacheDir, err := os.UserCacheDir()
	// if err != nil {
	// 	return uri, err
	// }
	baseCacheDir := filepath.Join(os.Getenv("HOME"), ".cache")

	cacheDir := filepath.Join(baseCacheDir, "kni-install", "libvirt")
	httpCacheDir := filepath.Join(cacheDir, "http")
	err := os.MkdirAll(httpCacheDir, 0777)
	if err != nil {
		return uri, err
	}

	cache := diskcache.NewWithDiskv(diskv.New(diskv.Options{
		BasePath:     httpCacheDir,
		CacheSizeMax: 0, // This stops the diskcache from caching the resp in memory.
	}))
	transport := httpcache.NewTransport(cache)
	resp, err := transport.Client().Get(uri)
	if err != nil {
		return uri, err
	}
	if resp.StatusCode != 200 {
		return uri, fmt.Errorf("%s while getting %s", resp.Status, uri)
	}
	defer resp.Body.Close()

	key, err := cacheKey(resp.Header.Get("ETag"))
	if err != nil {
		return uri, fmt.Errorf("invalid ETag for %s: %v", uri, err)
	}

	imageCacheDir := filepath.Join(cacheDir, "image")
	err = os.MkdirAll(imageCacheDir, 0777)
	if err != nil {
		return uri, err
	}

	imagePath := filepath.Join(imageCacheDir, key)
	_, err = os.Stat(imagePath)
	if err == nil {
		logrus.Debugf("Using cached OS image %q", imagePath)
	} else {
		if !os.IsNotExist(err) {
			return uri, err
		}

		err = cacheImage(resp.Body, imagePath)
		if err != nil {
			return uri, err
		}
	}

	return fmt.Sprintf("file://%s", filepath.ToSlash(imagePath)), nil
}

func cacheKey(etag string) (key string, err error) {
	if etag == "" {
		return "", fmt.Errorf("caching is not supported when ETag is unset")
	}
	etagSections := strings.SplitN(etag, "\"", 3)
	if len(etagSections) != 3 {
		return "", fmt.Errorf("broken quoting: %s", etag)
	}
	if etagSections[0] == "W/" {
		return "", fmt.Errorf("caching is not supported for weak ETags: %s", etag)
	}
	opaque := etagSections[1]
	if opaque == "" {
		return "", fmt.Errorf("caching is not supported when the opaque tag is unset: %s", etag)
	}
	hashed := md5.Sum([]byte(opaque))
	return hex.EncodeToString(hashed[:]), nil
}

func cacheImage(reader io.Reader, imagePath string) (err error) {
	logrus.Debugf("Unpacking OS image into %q...", imagePath)

	flockPath := fmt.Sprintf("%s.lock", imagePath)
	flock, err := os.Create(flockPath)
	if err != nil {
		return err
	}
	defer flock.Close()
	defer func() {
		err2 := os.Remove(flockPath)
		if err == nil {
			err = err2
		}
	}()

	err = unix.Flock(int(flock.Fd()), unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer func() {
		err2 := unix.Flock(int(flock.Fd()), unix.LOCK_UN)
		if err == nil {
			err = err2
		}
	}()

	_, err = os.Stat(imagePath)
	if err != nil && !os.IsNotExist(err) {
		return nil // another cacheImage beat us to it
	}

	tempPath := fmt.Sprintf("%s.tmp", imagePath)
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			file.Close()
		}
	}()

	_, err = io.Copy(file, reader)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
	closed = true

	return os.Rename(tempPath, imagePath)
}
