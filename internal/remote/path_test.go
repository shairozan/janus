package remote

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shairozan/janus/internal/config"
)

func mapper() *PathMapper {
	return NewPathMapper([]config.RemoteMount{
		{Local: `Z:\projects`, Remote: "/home/jane/projects"},
		{Local: `Z:\projects\sub`, Remote: "/mnt/fast/sub"},
		{Local: "/local/models", Remote: "/remote/models"},
	})
}

func TestToRemote(t *testing.T) {
	m := mapper()

	assert.Equal(t, "/home/jane/projects/run1/model.mod", m.ToRemote(`Z:\projects\run1\model.mod`))
	// Forward-slash local input also maps.
	assert.Equal(t, "/remote/models/a.mod", m.ToRemote("/local/models/a.mod"))
}

func TestToRemoteLongestMountWins(t *testing.T) {
	m := mapper()
	// Z:\projects\sub is more specific than Z:\projects.
	assert.Equal(t, "/mnt/fast/sub/run1/model.mod", m.ToRemote(`Z:\projects\sub\run1\model.mod`))
}

func TestToRemoteNoMountIsPassthrough(t *testing.T) {
	m := mapper()
	assert.Equal(t, `C:\elsewhere\x.mod`, m.ToRemote(`C:\elsewhere\x.mod`))
}

func TestToLocal(t *testing.T) {
	m := mapper()

	want := filepath.FromSlash("Z:/projects/run1/model.lst")
	assert.Equal(t, want, m.ToLocal("/home/jane/projects/run1/model.lst"))
}

func TestToLocalLongestMountWins(t *testing.T) {
	m := mapper()

	want := filepath.FromSlash("Z:/projects/sub/run1/model.lst")
	assert.Equal(t, want, m.ToLocal("/mnt/fast/sub/run1/model.lst"))
}

func TestHasMounts(t *testing.T) {
	assert.True(t, mapper().HasMounts())
	assert.False(t, NewPathMapper(nil).HasMounts())
}

func TestMountBoundaryNotPrefixSubstring(t *testing.T) {
	// `Z:\projectsX` must not match the `Z:\projects` mount.
	m := NewPathMapper([]config.RemoteMount{{Local: `Z:\projects`, Remote: "/r"}})
	assert.Equal(t, `Z:\projectsX\a.mod`, m.ToRemote(`Z:\projectsX\a.mod`))
}
