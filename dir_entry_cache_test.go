package adbfs

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
)

type delegateDirEntryCache struct {
	DoGetOrLoad        func(path string, loader DirEntryLoader) (entries *CachedDirEntries, err error, hit bool)
	DoGet              func(path string) (entries *CachedDirEntries, found bool)
	DoRemoveEventually func(path string)
}

func (c *delegateDirEntryCache) GetOrLoad(path string, loader DirEntryLoader) (entries *CachedDirEntries, err error, hit bool) {
	return c.DoGetOrLoad(path, loader)
}

func (c *delegateDirEntryCache) Get(path string) (entries *CachedDirEntries, found bool) {
	return c.DoGet(path)
}

func (c *delegateDirEntryCache) RemoveEventually(path string) {
	c.DoRemoveEventually(path)
}

func TestDirEntryCacheLoadSuccess(t *testing.T) {
	cache := NewDirEntryCache(5 * time.Second)
	loader := func(path string) (*CachedDirEntries, error) {
		return &CachedDirEntries{
			InOrder: []*adb.DirEntry{&adb.DirEntry{
				Name: path,
			}}}, nil
	}

	entries, err, hit := cache.GetOrLoad("foobar", loader)

	assert.NoError(t, err)
	assert.Len(t, entries.InOrder, 1)
	assert.False(t, hit)
	assert.Equal(t, "foobar", entries.InOrder[0].Name)
}

func TestDirEntryCacheLoadFail(t *testing.T) {
	cache := NewDirEntryCache(5 * time.Second)
	loader := func(path string) (*CachedDirEntries, error) {
		return nil, errors.New("the fail")
	}

	entries, err, hit := cache.GetOrLoad("foobar", loader)

	assert.EqualError(t, err, "the fail")
	assert.False(t, hit)
	assert.Nil(t, entries)
}

func TestDirEntryCacheHit(t *testing.T) {
	cache := NewDirEntryCache(5 * time.Second)
	loadCount := 0
	loader := func(path string) (entries *CachedDirEntries, err error) {
		loadCount++
		return
	}

	_, _, hit := cache.GetOrLoad("foobar", loader)
	assert.False(t, hit)
	assert.Equal(t, 1, loadCount)
	_, found := cache.Get("foobar")
	assert.True(t, found)

	_, _, hit = cache.GetOrLoad("foobar", loader)
	assert.Equal(t, 1, loadCount)
	assert.True(t, hit)
}

func TestDirEntryCacheMiss(t *testing.T) {
	cache := NewDirEntryCache(5 * time.Second)
	_, found := cache.Get("foobar")
	assert.False(t, found)
}

func TestDirEntryCacheExpiry(t *testing.T) {
	ttl := 10 * time.Millisecond
	cache := NewDirEntryCache(ttl)
	loadCount := 0
	loader := func(path string) (entries *CachedDirEntries, err error) {
		loadCount++
		return
	}

	cache.GetOrLoad("foobar", loader)
	assert.Equal(t, 1, loadCount)

	time.Sleep(ttl)

	_, found := cache.Get("foobar")
	assert.False(t, found)

	cache.GetOrLoad("foobar", loader)
	assert.Equal(t, 2, loadCount)
}
