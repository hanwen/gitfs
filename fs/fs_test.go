package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"

	git "github.com/libgit2/git2go"
)

func setupRepo(dir string) (*git.Repository, error) {
	repo, err := git.InitRepository(dir, false)
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

	if err = subTree.Insert("subfile", blobId, git.FilemodeBlobExecutable); err != nil {
		return nil, err
	}
	treeId, err := subTree.Write()
	if err != nil {
		return nil, err
	}

	rootTree, err := repo.TreeBuilder()
	if err != nil {
		return nil, err
	}
	defer rootTree.Free()

	if err := rootTree.Insert("dir", treeId, git.FilemodeTree); err != nil {
		return nil, err
	}
	if err := rootTree.Insert("file", blobId, git.FilemodeBlob); err != nil {
		return nil, err
	}
	if err = rootTree.Insert("link", blobId, git.FilemodeLink); err != nil {
		return nil, err
	}

	rootId, err := rootTree.Write()
	if err != nil {
		return nil, err
	}

	root, err := repo.LookupTree(rootId)
	if err != nil {
		return nil, err
	}

	sig := &git.Signature{"user", "user@invalid", time.Now()}
	if _, err := repo.CreateCommit("refs/heads/master", sig, sig,
		"message", root); err != nil {
		return nil, err
	}

	return repo, nil
}

type testCase struct {
	repo   *git.Repository
	server *fuse.Server
	mnt    string
}

func (tc *testCase) Cleanup() {
	tc.server.Unmount()
	tc.repo.Free()
}

func setupBasic(opts *GitFSOptions) (*testCase, error) {
	dir, err := ioutil.TempDir("", "fs_test")
	if err != nil {
		return nil, err
	}

	repo, err := setupRepo(filepath.Join(dir, "repo"))
	if err != nil {
		return nil, err
	}

	root, err := NewTreeFSRoot(repo, "refs/heads/master", nil)
	if err != nil {
		return nil, err
	}

	mnt := filepath.Join(dir, "mnt")
	if err := os.Mkdir(mnt, 0755); err != nil {
		return nil, err
	}

	server, _, err := nodefs.MountRoot(mnt, root, nil)
	server.SetDebug(true)
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
	tc, err := setupBasic(nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer tc.Cleanup()

	testGitFS(tc.mnt, t)
}

func TestBasicLazy(t *testing.T) {
	tc, err := setupBasic(&GitFSOptions{Disk: true})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer tc.Cleanup()

	testGitFS(tc.mnt, t)
}

func TestSymlink(t *testing.T) {
	tc, err := setupBasic(nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer tc.Cleanup()

	if err := os.Symlink("content", tc.mnt+"/mylink"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if content, err := os.Readlink(tc.mnt + "/mylink"); err != nil {
		t.Fatalf("Readlink: %v", err)
	} else if content != "content" {
		t.Fatalf("got %q, want %q", content, "content")
	}

	if err := os.Remove(tc.mnt + "/link"); err == nil {
		t.Fatalf("removed r/o file")
	}

	if err := os.Remove(tc.mnt + "/mylink"); err != nil {
		t.Fatalf("Remove: %v")
	}

	if fi, err := os.Lstat(tc.mnt + "/mylink"); err == nil {
		t.Fatalf("link still there: %v", fi)
	}
}

func testGitFS(mnt string, t *testing.T) {
	fi, err := os.Lstat(mnt + "/file")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if fi.IsDir() {
		t.Fatalf("got mode %v, want file", fi.Mode())
	} else if fi.Size() != 5 {
		t.Fatalf("got size %d, want file size 5", fi.Size())
	}

	if fi, err := os.Lstat(mnt + "/dir"); err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if !fi.IsDir() {
		t.Fatalf("got %v, want dir", fi)
	}

	if fi, err := os.Lstat(mnt + "/dir/subfile"); err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if fi.IsDir() || fi.Size() != 5 || fi.Mode()&0x111 == 0 {
		t.Fatalf("got %v, want +x file size 5", fi)
	}

	if fi, err := os.Lstat(mnt + "/link"); err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("got %v, want symlink", fi.Mode())
	}

	if content, err := ioutil.ReadFile(mnt + "/file"); err != nil {
		t.Fatalf("ReadFile: %v", err)
	} else if string(content) != "hello" {
		t.Errorf("got %q, want %q", content, "hello")
	}
}

func setupMulti() (*testCase, error) {
	dir, err := ioutil.TempDir("", "fs_test")
	if err != nil {
		return nil, err
	}

	repo, err := setupRepo(filepath.Join(dir, "repo"))
	if err != nil {
		return nil, err
	}

	root := NewMultiGitFSRoot(nil)
	if err != nil {
		return nil, err
	}

	mnt := filepath.Join(dir, "mnt")
	if err := os.Mkdir(mnt, 0755); err != nil {
		return nil, err
	}

	server, _, err := nodefs.MountRoot(mnt, root, nil)
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

func TestMultiFS(t *testing.T) {
	tc, err := setupMulti()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer tc.Cleanup()

	if err := os.Mkdir(tc.mnt+"/config/sub", 0755); err != nil {
		t.Fatalf("Mkdir %v", err)
	}

	if fi, err := os.Lstat(tc.mnt + "/sub"); err != nil {
		t.Fatalf("Lstat: %v", err)
	} else if !fi.IsDir() {
		t.Fatalf("want dir, got %v", fi.Mode())
	}

	if err := os.Symlink(tc.repo.Path()+":master", tc.mnt+"/config/sub/repo"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	entries, err := ioutil.ReadDir(tc.mnt + "/sub")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %v, want 2 entries", entries)
	}

	testGitFS(tc.mnt+"/sub/repo", t)

	// Ugh. the RELEASE opcode is not synchronized, so it
	// may not be completed while we try the unmount.
	time.Sleep(time.Millisecond)
	if err := os.Remove(tc.mnt + "/config/sub/repo"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Lstat(tc.mnt + "/sub/repo"); err == nil {
		t.Errorf("repo is still there.")
	}
}
