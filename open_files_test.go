package adbfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zach-klippenstein/goadb"
)

func TestOpenFiles_GetOrLoadSameFileSeparate(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/",
		}),
		openRead: openReadString("hello"),
	}
	o := NewOpenFiles(OpenFilesOptions{
		DeviceSerial:  "abc",
		ClientFactory: func() DeviceClient { return dev },
	})

	f1, err := o.GetOrLoad("/", O_RDONLY, 0, &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, 1, f1.RefCount())

	f1.DecRefCount()

	f2, err := o.GetOrLoad("/", O_RDONLY, 0, &LogEntry{})
	assert.NoError(t, err)
	assert.NotEqual(t, f1, f2)
	assert.Equal(t, 1, f2.RefCount())
	assert.Equal(t, 0, f1.RefCount())
}

func TestOpenFiles_GetOrLoadSameFileShared(t *testing.T) {
	dev := &delegateDeviceClient{
		stat: statFiles(&adb.DirEntry{
			Name: "/",
		}),
		openRead: openReadString("hello"),
	}
	o := NewOpenFiles(OpenFilesOptions{
		DeviceSerial:  "abc",
		ClientFactory: func() DeviceClient { return dev },
	})

	f1, err := o.GetOrLoad("/", O_RDONLY, 0, &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, 1, f1.RefCount())

	f2, err := o.GetOrLoad("/", O_RDONLY, 0, &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, f1, f2)
	assert.Equal(t, 2, f2.RefCount())
	assert.Equal(t, 2, f1.RefCount())

	f1.DecRefCount()
	assert.Equal(t, 1, f2.RefCount())
	assert.Equal(t, 1, f1.RefCount())

	f3, err := o.GetOrLoad("/", O_RDONLY, 0, &LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, f2, f3)
	assert.Equal(t, 2, f3.RefCount())
	assert.Equal(t, 2, f2.RefCount())
}
