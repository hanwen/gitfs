package fs

import (
//	"github.com/hanwen/go-fuse/fuse"
	"testing"
	"io/ioutil"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	git "github.com/libgit2/git2go"
)

func TestBasic(t *testing.T) {
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("TempDir:%v", err)
	}

	repo, err := git.OpenRepository("/home/hanwen/go/src/go-fuse")
	if err != nil {
		t.Fatalf("OpenRepository:%v", err)
	}

	fs, err := NewTreeFS(repo, "refs/heads/master")
	if err != nil {
		t.Fatalf(":%v", err)
	}

	server, _, err := nodefs.MountFileSystem(d, fs, nil)
	if err != nil {
		t.Fatalf(":%v", err)
	}

	go server.Serve()
	defer server.Unmount()

	entries, err := ioutil.ReadDir(d)
	if err != nil {
		t.Fatalf(":%v", err)
	}

	for _, e := range entries {
		t.Log(e)
	}
}
