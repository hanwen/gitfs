package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hanwen/gitfs/fs"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

func main() {
	lazy := flag.Bool("lazy", true, "only read contents for reads")
	disk := flag.Bool("disk", false, "don't use intermediate files")
	repo := flag.String("repo", "", "if set, mount a single repository.")
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatalf("usage: %s MOUNT", os.Args[0])
	}

	mntDir := flag.Args()[0]
	opts := fs.GitFSOptions{
		Lazy: *lazy,
		Disk: *disk,
	}
	var root nodefs.Node
	if *repo != "" {
		components := strings.Split(*repo, ":")
		if len(components) != 2 {
			log.Fatalf("must have 2 components: %q", *repo)
		}

		var err error
		root, err = fs.NewGitFSRoot(*repo, &opts)
		if err  != nil {
			log.Fatalf("NewGitFSRoot: %v", err)
		}
	} else {
		root = fs.NewMultiGitFSRoot(&opts)
	}
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
