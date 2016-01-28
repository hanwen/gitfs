package fs

import (
	"log"
	"sync"
	"path/filepath"

	git "github.com/libgit2/git2go"
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

func NewManifestFS(m *manifest.Manifest, repoRoot string, gitOpts *GitFSOptions) (nodefs.Node, error) {
	filtered := *m
	filtered.Project = nil
	for _, p := range m.Project {
		if p.Groups["notdefault"] {
			continue
		}
		filtered.Project = append(filtered.Project, p)
	}
		
	root := &manifestFSRoot{
		Node:     nodefs.NewDefaultNode(),
		repoMap:  map[string]nodefs.Node{},
		manifest: filtered,
	}

	type result struct {
		name string
		node nodefs.Node
		err error
	}

	ch := make(chan result, len(root.manifest.Project))
	for _, p := range root.manifest.Project {
		go func (p manifest.Project) {
			// the spec isn't clear about this, but the git repo
			// is placed locally at p.Path rather than p.Name
			repo, err := git.OpenRepository(filepath.Join(repoRoot, p.Path) + ".git")
			if err != nil {
				ch <- result{err: err}
				return
			}

			remote := p.Remote
			revision := p.Revision
			if revision == "" {
				revision = root.manifest.Default.Revision
			}
			if remote == "" {
				remote = root.manifest.Remote.Name
			}

			commit := filepath.Join(remote, revision)
			projectRoot, err := NewTreeFSRoot(repo, commit, gitOpts)
			ch <- result{p.Name, projectRoot, err}
		}(p)
	}

	var firstError error
	for _ = range root.manifest.Project {
		res := <-ch
		if firstError != nil {
			continue
		}
		if res.err != nil {
			firstError = res.err
		} else {
			root.repoMap[res.name]  = res.node
		}
	}
	if firstError != nil {
		return nil, firstError
	}
	return root, nil
}

func parents(path string) []string {
	var r []string
	for {
		path = filepath.Dir(path)
		if path == "." {
			break
		}
		r = append(r, path)
	}
	return r
}


func (r *manifestFSRoot) OnMount(fsConn *nodefs.FileSystemConnector) {
	r.fsConn = fsConn

	todo := map[string]manifest.Project{}
	for _, project := range r.manifest.Project {
		if project.Groups["notdefault"] {
			continue
		}

		todo[project.Path] = project
	}
	
	for len(todo) > 0 {
		next := map[string]manifest.Project{}
		var wg sync.WaitGroup
		for _, t := range todo {
			foundParent := false
			for _, p := range parents(t.Path) {
				if _, ok := todo[p] ; ok {
					foundParent = true
					break
				}
			}

			if !foundParent {
				wg.Add(1)
				go func(p manifest.Project) {
					r.addRepo(&p)
					wg.Done()
				}(t)
			} else {
				next[t.Path] = t
			}
		}
		wg.Wait()
		todo = next
	}
}

func (r *manifestFSRoot) addRepo(project *manifest.Project) {
	node, components := r.fsConn.Node(r.Inode(), project.Path)
	if len(components) == 0 {
		log.Fatalf("huh %v", *project)
	}
	last := len(components) - 1
	for _, c := range components[:last] {
		node = node.NewChild(c, true, nodefs.NewDefaultNode())
	}

	rootNode := r.repoMap[project.Name]
	if rootNode == nil {
		panic(project.Name)
	}
	if code := r.fsConn.Mount(node, components[last], rootNode, nil); !code.Ok() {
		// TODO - this cannot happen if the manifest
		// is well formed, but should check that in a
		// place where we can return error.
		log.Printf("Mount: %v - %v", project, code)
	}
}
