package main

import (
	"log"
	"flag"
	"os"
	"time"
	
	"github.com/hanwen/gitfs/fs"
	"github.com/hanwen/go-fuse/fuse/nodefs"

	git "github.com/libgit2/git2go"
)

func main() {
	tree := flag.String("tree", "master", "tree to mount")
	flag.Parse()
	
	if len(flag.Args()) < 2 {
		log.Fatalf("usage: %s REPO MOUNT", os.Args[0])
	}

	repoDir := flag.Args()[0]
	mntDir := flag.Args()[1]

	repo, err := git.OpenRepository(repoDir)
	if err != nil {
		log.Fatalf("OpenRepository(%q): %v", repoDir, err)
	}

	fs, err := fs.NewTreeFS(repo, *tree)
	if err != nil {
		log.Fatalf("NewTreeFS(%q): %v", *tree, err)
	}

	server, _, err := nodefs.MountFileSystem(mntDir, fs, &nodefs.Options{
		EntryTimeout: time.Hour,
		NegativeTimeout: time.Hour,
		AttrTimeout: time.Hour,
		PortableInodes: true,
	})
	log.Printf("Started gitfs FUSE on %s", mntDir)
	server.Serve()
}
