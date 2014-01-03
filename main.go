package main

import (
	"log"
	"flag"
	"os"
	"time"
	
	"github.com/hanwen/gitfs/fs"
	git "github.com/libgit2/git2go"
	
	"github.com/hanwen/go-fuse/fuse/nodefs"
)


func main() {
	branch := flag.String("branch", "master", "branch to open")
	flag.Parse()
	
	if len(flag.Args()) < 2 {
		log.Fatalf("usage: %s REPO MOUNT", os.Args[0])
	}

	repoDir := os.Args[1]
	mntDir := os.Args[2]

	repo, err := git.OpenRepository(repoDir)
	if err != nil {
		log.Fatalf("OpenRepository(%q): %v", repoDir, err)
	}

	*branch = "refs/heads/" + *branch
	fs, err := fs.NewTreeFS(repo, *branch)
	if err != nil {
		log.Fatalf("NewTreeFS(%q): %v", *branch, err)
	}

	server, _, err := nodefs.MountFileSystem(mntDir, fs, &nodefs.Options{
		EntryTimeout: time.Hour,
		NegativeTimeout: time.Hour,
		AttrTimeout: time.Hour,
		PortableInodes: true,
	})

	server.Serve()
}
