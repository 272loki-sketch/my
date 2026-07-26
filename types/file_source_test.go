package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLSource(t *testing.T) {
	t.Parallel()

	src := NewURLFileSource("https://example.com/image.png")
	require.True(t, src.IsURL())
	require.Equal(t, "https://example.com/image.png", src.GetIdentifier())
	require.Equal(t, "https://example.com/image.png", src.GetRawData())

	// ClearRawData is a no-op for URL sources
	src.ClearRawData()
	require.Equal(t, "https://example.com/image.png", src.GetRawData())

	long := NewURLFileSource("https://example.com/" + strings.Repeat("a", 200))
	id := long.GetIdentifier()
	require.Len(t, id, 103)
	require.True(t, strings.HasSuffix(id, "..."))
}

func TestBase64Source(t *testing.T) {
	t.Parallel()

	src := NewBase64FileSource("aGVsbG8=", "image/png")
	require.False(t, src.IsURL())
	require.Equal(t, "base64:aGVsbG8=", src.GetIdentifier())
	require.Equal(t, "aGVsbG8=", src.GetRawData())
	require.Equal(t, "image/png", src.MimeType)

	// small payloads are kept so they can still be reused
	src.ClearRawData()
	require.Equal(t, "aGVsbG8=", src.GetRawData())

	long := NewBase64FileSource(strings.Repeat("a", 2048), "image/png")
	id := long.GetIdentifier()
	require.Equal(t, "base64:"+strings.Repeat("a", 50)+"...", id)
	long.ClearRawData()
	require.Empty(t, long.GetRawData())
}

func TestNewFileSourceFromData(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		data  string
		isURL bool
	}{
		{name: "http url", data: "http://example.com/a.png", isURL: true},
		{name: "https url", data: "https://example.com/a.png", isURL: true},
		{name: "base64 payload", data: "aGVsbG8=", isURL: false},
		{name: "data uri is treated as base64", data: "data:image/png;base64,aGVsbG8=", isURL: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := NewFileSourceFromData(tc.data, "image/png")
			require.Equal(t, tc.isURL, src.IsURL())
			require.Equal(t, tc.data, src.GetRawData())
		})
	}
}

func TestFileSourceCacheAndRegistration(t *testing.T) {
	t.Parallel()

	src := NewURLFileSource("https://example.com/a.png")
	require.False(t, src.HasCache())
	require.Nil(t, src.GetCache())
	require.False(t, src.IsRegistered())
	require.NotNil(t, src.Mu())

	src.SetRegistered(true)
	require.True(t, src.IsRegistered())

	cached := NewMemoryCachedData("aGVsbG8=", "image/png", 5)
	src.SetCache(cached)
	require.True(t, src.HasCache())
	require.Same(t, cached, src.GetCache())

	src.ClearCache()
	require.False(t, src.HasCache())
	require.Nil(t, src.GetCache())

	// clearing an already empty cache is safe
	src.ClearCache()
	require.False(t, src.HasCache())
}

func TestMemoryCachedData(t *testing.T) {
	t.Parallel()

	c := NewMemoryCachedData("aGVsbG8=", "image/png", 5)
	require.False(t, c.IsDisk())

	data, err := c.GetBase64Data()
	require.NoError(t, err)
	require.Equal(t, "aGVsbG8=", data)

	c.SetBase64Data("d29ybGQ=")
	data, err = c.GetBase64Data()
	require.NoError(t, err)
	require.Equal(t, "d29ybGQ=", data)

	require.NoError(t, c.Close())
	data, err = c.GetBase64Data()
	require.NoError(t, err)
	require.Empty(t, data)
}

func TestDiskCachedData(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "cache.txt")
	require.NoError(t, os.WriteFile(path, []byte("aGVsbG8="), 0o600))

	c := NewDiskCachedData(path, "image/png", 8)
	c.DiskSize = 8
	var closedSize int64
	c.OnClose = func(size int64) { closedSize = size }

	require.True(t, c.IsDisk())
	data, err := c.GetBase64Data()
	require.NoError(t, err)
	require.Equal(t, "aGVsbG8=", data)

	// disk-backed caches ignore in-memory writes
	c.SetBase64Data("d29ybGQ=")
	data, err = c.GetBase64Data()
	require.NoError(t, err)
	require.Equal(t, "aGVsbG8=", data)

	require.NoError(t, c.Close())
	require.Equal(t, int64(8), closedSize)
	require.NoFileExists(t, path)

	// closing twice is a no-op and does not decrement stats again
	closedSize = 0
	require.NoError(t, c.Close())
	require.Equal(t, int64(0), closedSize)

	_, err = c.GetBase64Data()
	require.ErrorContains(t, err, "already closed")
}

func TestDiskCachedDataMissingFile(t *testing.T) {
	t.Parallel()

	c := NewDiskCachedData(filepath.Join(t.TempDir(), "missing.txt"), "image/png", 0)
	_, err := c.GetBase64Data()
	require.ErrorContains(t, err, "failed to read from disk cache")
}
