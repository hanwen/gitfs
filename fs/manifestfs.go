package fs

import (
	"fmt"
	"log"

	git "github.com/libgit2/git2go"
	"path/filepath"

	"github.com/hanwen/gitfs/manifest"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type manifestFSRoot struct {
	nodefs.Node

	manifest manifest.Manifest
	fsConn   *nodefs.FileSystemConnector
	// keyed by name (from the manifest)
	repoMap map[string]nodefs.Node
}

func NewManifestFS(m *manifest.Manifest, repoRoot string) (nodefs.Node, error) {
	root := &manifestFSRoot{
		Node:     nodefs.NewDefaultNode(),
		repoMap:  map[string]nodefs.Node{},
		manifest: *m,
	}
	for _, p := range m.Project {
		if p.Groups["notdefault"] {
			continue
		}
		// the spec isn't clear about this, but the git repo
		// is placed locally at p.Path rather than p.Name
		repo, err := git.OpenRepository(filepath.Join(repoRoot, p.Path) + ".git")
		if err != nil {
			return nil, err
		}

		remote := p.Remote
		revision := p.Revision
		if revision == "" {
			revision = m.Default.Revision
		}
		if remote == "" {
			remote = m.Remote.Name
		}

		commit := filepath.Join(remote, revision)
		projectRoot, err := NewTreeFSRoot(repo, commit, nil)
		if err != nil {
			// TODO - resource leak.
			return nil, fmt.Errorf("NewTreeFSRoot(%q, %q): %v",
				repo.Path(), commit, err)
		}
		root.repoMap[p.Name] = projectRoot
	}

	return root, nil
}

func (r *manifestFSRoot) OnMount(fsConn *nodefs.FileSystemConnector) {
	r.fsConn = fsConn

	for _, project := range r.manifest.Project {
		if project.Groups["notdefault"] {
			continue
		}

		node, components := fsConn.Node(r.Inode(), project.Path)
		if len(components) == 0 {
			panic("huh?")
		}
		last := len(components) - 1
		for _, c := range components[:last] {
			node = node.NewChild(c, true, nodefs.NewDefaultNode())
		}

		if code := fsConn.Mount(node, components[last], r.repoMap[project.Name], nil); !code.Ok() {
			// TODO - this cannot happen if the manifest
			// is well formed, but should check that in a
			// place where we can return error.
			log.Printf("Mount: %v - %v", project, code)
		}
	}
}
