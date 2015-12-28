package adbfs

import (
	"io"
	"os"
	"path"
	"time"

	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

type CachingDeviceClient struct {
	DeviceClient
	Cache DirEntryCache
}

type CachedDirEntries struct {
	InOrder []*goadb.DirEntry
	ByName  map[string]*goadb.DirEntry
}

func NewCachingDeviceClientFactory(cache DirEntryCache, factory DeviceClientFactory) DeviceClientFactory {
	return func() DeviceClient {
		return &CachingDeviceClient{
			DeviceClient: factory(),
			Cache:        cache,
		}
	}
}

func NewCachedDirEntries(entries []*goadb.DirEntry) *CachedDirEntries {
	result := &CachedDirEntries{
		InOrder: entries,
		ByName:  make(map[string]*goadb.DirEntry),
	}

	for _, entry := range result.InOrder {
		result.ByName[entry.Name] = entry
	}

	return result
}

func (c *CachingDeviceClient) Stat(name string, log *LogEntry) (*goadb.DirEntry, error) {
	dir := path.Dir(name)
	base := path.Base(name)

	if dir == base {
		// Don't ask the cache for the root stat, we never cache the root.
		return c.DeviceClient.Stat(name, log)
	}

	if entries, found := c.Cache.Get(dir); found {
		log.CacheUsed(true)

		if entry, found := entries.ByName[base]; found {
			return entry, nil
		}

		// Cached directory list doesn't have name, so as far as we're concerned the
		// file doesn't exist.
		return nil, util.Errorf(util.FileNoExistError,
			"name '%s' does not exist in cached directory listing", base)
	}
	log.CacheUsed(false)

	// The directory doesn't exist in the cache, so perform a one-off lookup on the device.
	return c.DeviceClient.Stat(name, log)
}

func (c *CachingDeviceClient) ListDirEntries(path string, log *LogEntry) ([]*goadb.DirEntry, error) {
	entries, err, hit := c.Cache.GetOrLoad(path, func(path string) (*CachedDirEntries, error) {
		entries, err := c.DeviceClient.ListDirEntries(path, log)
		if err != nil {
			return nil, err
		}
		return NewCachedDirEntries(entries), nil
	})
	log.CacheUsed(hit)

	if err != nil {
		return nil, err
	}
	return entries.InOrder, nil
}

func (c *CachingDeviceClient) OpenWrite(name string, perms os.FileMode, mtime time.Time, log *LogEntry) (io.WriteCloser, error) {
	// Writing to the file obviously invalidates the file's cache entry.
	w, err := c.DeviceClient.OpenWrite(name, perms, mtime, log)

	// The mtime is only set on the file on close, so don't bother invalidating the cache until then.
	onClosed := func() {
		c.Cache.RemoveEventually(path.Dir(name))
	}
	return onCloseWriter{w, onClosed}, err
}

type onCloseWriter struct {
	io.WriteCloser
	onClosed func()
}

func (w onCloseWriter) Close() (err error) {
	err = w.WriteCloser.Close()
	w.onClosed()
	return
}
