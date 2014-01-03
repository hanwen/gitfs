package fs

import (
	"log"
	"time"
	"syscall"
	"strings"
	"os"
	
	git "github.com/libgit2/git2go"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
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

	content string
}

func newGitConfigNode(content string) *gitConfigNode {
	return &gitConfigNode{
		Node: nodefs.NewDefaultNode(),
		content: content,
	}
}

func (n *gitConfigNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	out.Mode = syscall.S_IFLNK
	return fuse.OK
}

func (n *gitConfigNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	return []byte(n.content), fuse.OK
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

	root := n.corresponding.Inode().GetChild(name)
	if root == nil {
		return fuse.EINVAL
	}

	code = n.fs.fsConn.Unmount(root)
	if code.Ok() {
		n.Inode().RmChild(name)
	}
	return code
}

func (n *configNode) Symlink(name string, content string, context *fuse.Context) (newNode nodefs.Node, code fuse.Status) {
	dir := content
	components := strings.Split(content, ":")
	if len(components) > 2 || len(components) == 0{
		return nil, fuse.Status(syscall.EINVAL)
	}
	if len(components) == 2 {
		dir = components[0]
	}
	
	if fi, err := os.Lstat(dir); err != nil {
		return nil, fuse.ToStatus(err)
	} else if !fi.IsDir() {
		return nil, fuse.Status(syscall.ENOTDIR)
	}

	var opts *nodefs.Options
	var fs nodefs.FileSystem
	
	if len(components) == 1 {
		fs = pathfs.NewPathNodeFs(pathfs.NewLoopbackFileSystem(content), nil)
	} else {
		repo, err := git.OpenRepository(components[0])
		if err != nil {
			log.Printf("OpenRepository(%q): %v", components[0], err)
			return nil, fuse.ENOENT
		}
		
		fs, err = NewTreeFS(repo, components[1])
		if err != nil {
			log.Printf("NewTreeFS(%q): %v", components[1], err)
			return nil, fuse.ENOENT
		}

		opts = &nodefs.Options{
			EntryTimeout: time.Hour,
			NegativeTimeout: time.Hour,
			AttrTimeout: time.Hour,
			PortableInodes: true,
		}
	}
	
	if code := n.fs.fsConn.Mount(n.corresponding.Inode(), name, fs, opts); !code.Ok() {
		return nil, code
	}

	linkNode := newGitConfigNode(content)
	ch := n.Inode().New(false, linkNode)
	n.Inode().AddChild(name, ch)
	return linkNode, fuse.OK
}
