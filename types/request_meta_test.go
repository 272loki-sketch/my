package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFileMeta(t *testing.T) {
	t.Parallel()

	src := NewURLFileSource("https://example.com/a.mp3")
	meta := NewFileMeta(FileTypeAudio, src)

	require.Equal(t, FileTypeAudio, meta.FileType)
	require.Same(t, src, meta.Source)
	require.Empty(t, meta.Detail)
	require.True(t, meta.IsURL())
	require.Equal(t, "https://example.com/a.mp3", meta.GetIdentifier())
	require.Equal(t, "https://example.com/a.mp3", meta.GetRawData())
}

func TestNewImageFileMeta(t *testing.T) {
	t.Parallel()

	meta := NewImageFileMeta(NewBase64FileSource("aGVsbG8=", "image/png"), "high")

	require.Equal(t, FileTypeImage, meta.FileType)
	require.Equal(t, "high", meta.Detail)
	require.False(t, meta.IsURL())
	require.Equal(t, "base64:aGVsbG8=", meta.GetIdentifier())
	require.Equal(t, "aGVsbG8=", meta.GetRawData())
}

func TestFileMetaWithoutSource(t *testing.T) {
	t.Parallel()

	meta := &FileMeta{FileType: FileTypeFile}
	require.False(t, meta.IsURL())
	require.Equal(t, "unknown", meta.GetIdentifier())
	require.Empty(t, meta.GetRawData())
}
