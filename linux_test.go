//go:build linux
// +build linux

package lumberjack

import (
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestChown(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()

	t.Run("maintain_mode", func(t *testing.T) {
		currentTime = fakeTime
		dir := makeTempDir("TestMaintainMode", t)
		defer os.RemoveAll(dir)

		filename := logFile(dir)

		mode := os.FileMode(0600)
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
		isNil(err, t)
		f.Close()

		l := &Logger{
			Filename:   filename,
			MaxBackups: 1,
			MaxSize:    100, // megabytes
		}
		defer l.Close()
		b := []byte("boo!")
		n, err := l.Write(b)
		isNil(err, t)
		equals(len(b), n, t)

		newFakeTime()

		err = l.Rotate()
		isNil(err, t)

		filename2 := backupFile(dir)
		info, err := os.Stat(filename)
		isNil(err, t)
		info2, err := os.Stat(filename2)
		isNil(err, t)
		equals(mode, info.Mode(), t)
		equals(mode, info2.Mode(), t)
	})

	t.Run("maintain_owner", func(t *testing.T) {
		currentTime = fakeTime
		dir := makeTempDir("TestMaintainOwner", t)
		defer os.RemoveAll(dir)

		filename := logFile(dir)

		f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
		isNil(err, t)
		f.Close()

		l := &Logger{
			Filename:   filename,
			MaxBackups: 1,
			MaxSize:    100, // megabytes
		}
		defer l.Close()
		b := []byte("boo!")
		n, err := l.Write(b)
		isNil(err, t)
		equals(len(b), n, t)

		newFakeTime()

		err = l.Rotate()
		isNil(err, t)

		ff, ok := fakeFS.fakeFile(filename)
		if !ok {
			t.Fatalf("%q not found", filename)
		}
		equals(555, ff.uid, t)
		equals(666, ff.gid, t)
	})

	t.Run("compress_maintain_mode", func(t *testing.T) {
		currentTime = fakeTime

		dir := makeTempDir("TestCompressMaintainMode", t)
		defer os.RemoveAll(dir)

		filename := logFile(dir)

		mode := os.FileMode(0600)
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
		isNil(err, t)
		f.Close()

		l := &Logger{
			Compress:   true,
			Filename:   filename,
			MaxBackups: 1,
			MaxSize:    100, // megabytes
		}
		defer l.Close()
		b := []byte("boo!")
		n, err := l.Write(b)
		isNil(err, t)
		equals(len(b), n, t)

		newFakeTime()

		err = l.Rotate()
		isNil(err, t)

		// we need to wait a little bit since the files get compressed on a different
		// goroutine.
		<-time.After(10 * time.Millisecond)

		// a compressed version of the log file should now exist with the correct
		// mode.
		filename2 := backupFile(dir)
		info, err := os.Stat(filename)
		isNil(err, t)
		info2, err := os.Stat(filename2 + compressSuffix)
		isNil(err, t)
		equals(mode, info.Mode(), t)
		equals(mode, info2.Mode(), t)
	})

	t.Run("compress_maintain_owner", func(t *testing.T) {
		currentTime = fakeTime
		dir := makeTempDir("TestCompressMaintainOwner", t)
		defer os.RemoveAll(dir)

		filename := logFile(dir)

		f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
		isNil(err, t)
		f.Close()

		l := &Logger{
			Compress:   true,
			Filename:   filename,
			MaxBackups: 1,
			MaxSize:    100, // megabytes
		}
		defer l.Close()
		b := []byte("boo!")
		n, err := l.Write(b)
		isNil(err, t)
		equals(len(b), n, t)

		newFakeTime()

		err = l.Rotate()
		isNil(err, t)

		// we need to wait a little bit since the files get compressed on a different
		// goroutine.
		<-time.After(10 * time.Millisecond)

		// a compressed version of the log file should now exist with the correct
		// owner.
		filename2 := backupFile(dir)
		ff, ok := fakeFS.fakeFile(filename2 + compressSuffix)
		if !ok {
			t.Fatalf("%q not found", filename2+compressSuffix)
		}
		equals(555, ff.uid, t)
		equals(666, ff.gid, t)
	})
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	mu    sync.Mutex
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.mu.Lock()
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	fs.mu.Unlock()
	return nil
}

func (fs *fakeFS) fakeFile(name string) (f fakeFile, ok bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	f, ok = fs.files[name]
	return f, ok
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}
