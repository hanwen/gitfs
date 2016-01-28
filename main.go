package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hanwen/gitfs/fs"
	"github.com/hanwen/gitfs/manifest"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

func main() {
	lazy := flag.Bool("lazy", true, "only read contents for reads")
	disk := flag.Bool("disk", false, "don't use intermediate files")
	gitRepo := flag.String("git_repo", "", "if set, mount a single repository.")
	repo := flag.String("repo", "", "if set, mount a single manifest from repo repository.")
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
		xml := filepath.Join(*repo, "manifest.xml")

		m, err := manifest.ParseFile(xml)
		if err != nil {
			log.Fatalf("ParseFile(%q): %v", *repo, err)
		}

		root, err = fs.NewManifestFS(m, filepath.Join(*repo, "projects"))
		if err != nil {
			log.Fatalf("NewManifestFS: %v", err)
		}
	} else if *gitRepo != "" {
		var err error
		root, err = fs.NewGitFSRoot(*gitRepo, &opts)
		if err != nil {
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
