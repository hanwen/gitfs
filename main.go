package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/hanwen/gitfs/fs"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

func main() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatalf("usage: %s MOUNT", os.Args[0])
	}

	mntDir := flag.Args()[0]

	root := fs.NewMultiGitFSRoot()
	server, _, err := nodefs.MountRoot(mntDir, root, &nodefs.Options{
		EntryTimeout:    time.Hour,
		NegativeTimeout: time.Hour,
		AttrTimeout:     time.Hour,
		PortableInodes:  true,
	})
	if err != nil {
		log.Fatalf("MountFileSystem: %v", err)
	}
	log.Printf("Started git multi fs FUSE on %s", mntDir)
	server.Serve()
}
