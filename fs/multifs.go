package fs

import (
	"log"
	"time"
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

func (n *configNode) Mkdir(name string, mode uint32, context *fuse.Context) (nodefs.Node, fuse.Status) {
	corr := n.Inode().New(true, nodefs.NewDefaultNode())
	n.corresponding.Inode().AddChild(name, corr)
	c := n.fs.newConfigNode(corr.Node())
	n.Inode().AddChild(name, n.Inode().New(true, c))
	return c, fuse.OK
}

func (n *configNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	linkInode := n.Inode().GetChild(name)
	if linkInode == nil {
		return fuse.ENOENT
	}

	_, ok := linkInode.Node().(*gitConfigNode)
	if !ok {
		log.Printf("gitfs: removing %q, child is not a gitConfigNode", name)
		return fuse.EINVAL
	}

	gitFSNode := n.corresponding.Inode().GetChild(name)
	if gitFSNode == nil {
		return fuse.EINVAL
	}

	code = n.fs.fsConn.Unmount(gitFSNode)
	if code.Ok() {
		n.Inode().RmChild(name)
	}
	return code
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

	opts := &nodefs.Options{
		EntryTimeout: time.Hour,
		NegativeTimeout: time.Hour,
		AttrTimeout: time.Hour,
		PortableInodes: true,
	}
	if code := n.fs.fsConn.Mount(n.corresponding.Inode(), name, fs, opts); !code.Ok() {
		return nil, code
	}

	linkNode := newGitConfigNode(components[0], components[1])
	ch := n.Inode().New(false, linkNode)
	n.Inode().AddChild(name, ch)
	return linkNode, fuse.OK
}
