package adbfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
	"github.com/zach-klippenstein/goadb/util"
)

func TestNewCachedDirEntries(t *testing.T) {
	inOrder := []*goadb.DirEntry{
		&goadb.DirEntry{Name: "foo"},
		&goadb.DirEntry{Name: "bar"},
	}

	entries := NewCachedDirEntries(inOrder)

	assert.NotNil(t, entries)
	assert.Equal(t, inOrder, entries.InOrder)
	assert.Equal(t, inOrder[0], entries.ByName["foo"])
	assert.Equal(t, inOrder[1], entries.ByName["bar"])
}

func TestCachingDeviceClientStat_Miss(t *testing.T) {
	client := &CachingDeviceClient{
		DeviceClient: &delegateDeviceClient{
			stat: func(path string) (*goadb.DirEntry, error) {
				if path == "/foo/bar" {
					return &goadb.DirEntry{Name: "baz"}, nil
				}
				return nil, util.Errorf(util.FileNoExistError, "")
			},
		},
		Cache: &delegateDirEntryCache{
			DoGet: func(path string) (entries *CachedDirEntries, found bool) {
				return nil, false
			},
		},
	}

	entry, err := client.Stat("/foo/bar", &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "baz", entry.Name)
}

func TestCachingDeviceClientStat_HitExists(t *testing.T) {
	client := &CachingDeviceClient{
		DeviceClient: &delegateDeviceClient{},
		Cache: &delegateDirEntryCache{
			DoGet: func(path string) (entries *CachedDirEntries, found bool) {
				return NewCachedDirEntries([]*goadb.DirEntry{
					&goadb.DirEntry{Name: "bar"},
				}), true
			},
		},
	}

	entry, err := client.Stat("/foo/bar", &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "bar", entry.Name)
}

func TestCachingDeviceClientStat_HitNotExists(t *testing.T) {
	client := &CachingDeviceClient{
		DeviceClient: &delegateDeviceClient{},
		Cache: &delegateDirEntryCache{
			DoGet: func(path string) (entries *CachedDirEntries, found bool) {
				return NewCachedDirEntries([]*goadb.DirEntry{
					&goadb.DirEntry{Name: "baz"},
				}), true
			},
		},
	}

	_, err := client.Stat("/foo/bar", &LogEntry{})
	assert.True(t, util.HasErrCode(err, util.FileNoExistError))
}

func TestCachingDeviceClientStat_Root(t *testing.T) {
	client := &CachingDeviceClient{
		DeviceClient: &delegateDeviceClient{
			stat: func(path string) (*goadb.DirEntry, error) {
				if path == "/" {
					return &goadb.DirEntry{Name: "/"}, nil
				}
				return nil, util.Errorf(util.FileNoExistError, "")
			},
		},
		Cache: &delegateDirEntryCache{
			DoGet: func(path string) (entries *CachedDirEntries, found bool) {
				return NewCachedDirEntries([]*goadb.DirEntry{
					&goadb.DirEntry{Name: "bar"},
				}), true
			},
		},
	}

	entry, err := client.Stat("/", &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "/", entry.Name)
}
