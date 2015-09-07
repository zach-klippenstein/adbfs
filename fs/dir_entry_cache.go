package fs

import (
	"time"

	cache "github.com/pmylund/go-cache"
)

const CachePurgeInterval = 5 * time.Minute

type DirEntryLoader func(path string) (*CachedDirEntries, error)

// DirEntryCache is a key-value cache of normalized directory paths to
// slices of *goadb.FileEntries.
type DirEntryCache interface {
	GetOrLoad(path string, loader DirEntryLoader) (entries *CachedDirEntries, err error, hit bool)
	Get(path string) (entries *CachedDirEntries, found bool)
}

type realDirEntryCache cache.Cache

func NewDirEntryCache(ttl time.Duration) DirEntryCache {
	return (*realDirEntryCache)(cache.New(ttl, CachePurgeInterval))
}

func (c *realDirEntryCache) GetOrLoad(path string, loader DirEntryLoader) (*CachedDirEntries, error, bool) {
	if entries, found := c.Get(path); found {
		return entries, nil, true
	}

	entries, err := loader(path)
	if err != nil {
		return nil, err, false
	}

	c.Set(path, entries, cache.DefaultExpiration)
	return entries, nil, false
}

func (c *realDirEntryCache) Get(path string) (*CachedDirEntries, bool) {
	if entries, found := (*cache.Cache)(c).Get(path); found {
		return entries.(*CachedDirEntries), true
	}
	return nil, false
}
