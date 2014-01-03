package fs

import (
	"log"
	"syscall"
	"strings"
	
	git "github.com/libgit2/git2go"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse"
)

type multiGitFS struct {
	nodefs.FileSystem
	
	fsConn *nodefs.FileSystemConnector
	root    nodefs.Node
}

func NewMultiGitFS() *multiGitFS {
	return &multiGitFS{
		FileSystem: nodefs.NewDefaultFileSystem(),
		root: nodefs.NewDefaultNode(),
	}
}

func (m *multiGitFS) String() string {
	return "multigitfs"
}

func (m *multiGitFS) Root() nodefs.Node {
	return m.root
}

func (m *multiGitFS) OnMount(fsConn *nodefs.FileSystemConnector) {
	m.fsConn = fsConn
	config := m.root.Inode().New(true, m.newConfigNode(m.root))
	
	m.root.Inode().AddChild("config", config)
}


type configNode struct {
	fs *multiGitFS
	
	nodefs.Node

	// non-config node corresponding to this one.
	corresponding nodefs.Node
}

func (fs *multiGitFS) newConfigNode(corresponding nodefs.Node) *configNode {
	return &configNode{
		fs: fs,
		Node: nodefs.NewDefaultNode(),
		corresponding: corresponding,
	}
}

type gitConfigNode struct {
	nodefs.Node

	repo string
	treeish string
}

func newGitConfigNode(repo, tree string) *gitConfigNode {
	return &gitConfigNode{
		Node: nodefs.NewDefaultNode(),
		repo: repo,
		treeish: tree,
	}
}

func (n *gitConfigNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	out.Mode = syscall.S_IFLNK
	return fuse.OK
}

func (n *gitConfigNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	return []byte(n.repo + ":" + n.treeish), fuse.OK
}

func (n *configNode) Symlink(name string, content string, context *fuse.Context) (newNode nodefs.Node, code fuse.Status) {
	components := strings.Split(content, ":")
	if len(components) != 2 {
		return nil, fuse.Status(syscall.EINVAL)
	}
	
	repo, err := git.OpenRepository(components[0])
	if err != nil {
		log.Printf("OpenRepository(%q): %v", components[0], err)
		return nil, fuse.ENOENT
	}
	
	fs, err := NewTreeFS(repo, components[1])
	if err != nil {
		log.Printf("NewTreeFS(%q): %v", components[1], err)
		return nil, fuse.ENOENT
	}

	log.Println("mounting", components)
	// TODO - inherit options.
	if code := n.fs.fsConn.Mount(n.corresponding.Inode(), name, fs, nil); !code.Ok() {
		return nil, code
	}

	linkNode := newGitConfigNode(components[0], components[1])
	ch := n.Inode().New(false, linkNode)
	n.Inode().AddChild(name, ch)
	return linkNode, fuse.OK
}
