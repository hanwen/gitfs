package fs

import (
//	"github.com/hanwen/go-fuse/fuse"
	"testing"
	"io/ioutil"
	"path/filepath"
	"time"
	"os"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse"
	
	git "github.com/libgit2/git2go"
)

type testCase struct {
	repo *git.Repository
	server *fuse.Server
	mnt string 
}

func (tc *testCase) Cleanup() {
	tc.server.Unmount()
	tc.repo.Free()
}

func setup() (*testCase, error) {
	dir, err := ioutil.TempDir("", "fs_test")
	if err != nil {
		return nil, err
	}

	r := filepath.Join(dir, "repo")
	repo, err := git.InitRepository(r, false)
	if err != nil {
		return nil, err
	}
	odb, err := repo.Odb()
	if err != nil {
		return nil, err
	}
	
	blobId, err := odb.Write([]byte("hello"), git.ObjectBlob)
	if err != nil {
		return nil, err
	}

	subTree, err := repo.TreeBuilder()
	if err != nil {
		return nil, err
	}
	defer subTree.Free()
	
	if err = subTree.Insert("subfile", blobId, git.FilemodeBlobExecutable); err !=  nil {
		return nil, err
	}
	treeId, err := subTree.Write()
	if err !=  nil {
		return nil, err
	}

	rootTree, err := repo.TreeBuilder()
	if err !=  nil {
		return nil, err
	}
	defer rootTree.Free()

	if err := rootTree.Insert("dir", treeId, git.FilemodeTree); err !=  nil {
		return nil, err
	}
	if err := rootTree.Insert("file", blobId, git.FilemodeBlob); err !=  nil {
		return nil, err
	}
	if err = rootTree.Insert("link", blobId, git.FilemodeLink); err != nil {
		return nil, err
	}
	
	rootId, err := rootTree.Write()
	if err !=  nil {
		return nil, err
	}

	root, err := repo.LookupTree(rootId)
	if err !=  nil {
		return nil, err
	}
	
	sig := &git.Signature{"user", "user@invalid", time.Now()}
	if _, err := repo.CreateCommit("refs/heads/master", sig, sig,
		"message", root); err !=  nil {
		return nil, err
	}
	
	fs, err := NewTreeFS(repo, "refs/heads/master")
	if err != nil {
		return nil, err
	}
	
	mnt := filepath.Join(dir, "mnt")
	if err := os.Mkdir(mnt, 0755); err != nil {
		return nil, err
	}
	
	server, _, err := nodefs.MountFileSystem(mnt, fs, nil)
	go server.Serve()
	if err != nil {
		return nil, err
	}
	
	return &testCase{
		repo,
		server,
		mnt,
	}, nil
}

func TestBasic(t *testing.T) {
	tc, err := setup()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	fi, err := os.Lstat(tc.mnt + "/file")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if fi.IsDir() {
		t.Fatalf("got mode %v, want file", fi.Mode())
	} else if fi.Size() != 5 {
		t.Fatalf("got size %d, want file size 5", fi.Size())
	}
	
	if fi, err := os.Lstat(tc.mnt + "/dir"); err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if !fi.IsDir() {
		t.Fatalf("got %v, want dir", fi)
	}

	if fi, err := os.Lstat(tc.mnt + "/dir/subfile");  err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if fi.IsDir() || fi.Size() != 5 || fi.Mode() & 0x111 == 0 {
		t.Fatalf("got %v, want +x file size 5", fi)
	}

	if fi, err := os.Lstat(tc.mnt + "/link");  err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if fi.Mode() & os.ModeSymlink == 0 {
		t.Fatalf("got %v, want symlink", fi.Mode())
	}

	if content, err := ioutil.ReadFile(tc.mnt + "/file"); err != nil {
		t.Fatalf("ReadFile: %v", err)
	} else if string(content) != "hello" {
		t.Errorf("got %q, want %q", content, "hello")
	}
}
