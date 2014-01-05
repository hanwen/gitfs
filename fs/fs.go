package fs

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"syscall"

	git "github.com/libgit2/git2go"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type treeFS struct {
	repo *git.Repository
	dir  string
}

// NewTreeFS creates a git Tree FS. The treeish should resolve to tree SHA1.
func NewTreeFSRoot(repo *git.Repository, treeish string) (nodefs.Node, error) {
	obj, err := repo.RevparseSingle(treeish)
	if err != nil {
		return nil, err
	}
	defer obj.Free()

	var treeId *git.Oid
	switch obj.Type() {
	case git.ObjectCommit:
		commit, err := repo.LookupCommit(obj.Id())
		if err != nil {
			return nil, err
		}
		treeId = commit.TreeId()
	case git.ObjectTree:
		treeId = obj.Id()
	default:
		return nil, fmt.Errorf("gitfs: unsupported object type %d", obj.Type())
	}

	dir, err := ioutil.TempDir("", "gitfs")
	if err != nil {
		return nil, err
	}

	t := &treeFS{
		repo: repo,
		dir:  dir,
	}
	root := t.newDirNode(treeId)
	return root, nil
}

func (t *treeFS) onMount(root *dirNode) {
	tree, err := t.repo.LookupTree(root.id)
	if err != nil {
		panic(err)
	}

	if root.Inode() == nil {
		panic("nil?")
	}
	t.recurse(tree, root)
	if err != nil {
		panic(err)
	}
}

type mutableLink struct {
	nodefs.Node
	content []byte
}

func (n *mutableLink) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	out.Mode = fuse.S_IFLNK
	return fuse.OK
}

func (n *mutableLink) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	return n.content, fuse.OK
}

type gitNode struct {
	fs *treeFS
	id *git.Oid
	nodefs.Node
}

type dirNode struct {
	gitNode
}

func (n *dirNode) OnMount(conn *nodefs.FileSystemConnector) {
	n.fs.onMount(n)
}

func (n *dirNode) Symlink(name string, content string, context *fuse.Context) (newNode nodefs.Node, code fuse.Status) {
	log.Println("name", name)
	l := &mutableLink{nodefs.NewDefaultNode(), []byte(content)}
	n.Inode().NewChild(name, false, l)
	return l, fuse.OK
}

func (n *dirNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	ch := n.Inode().GetChild(name)
	if ch == nil {
		return fuse.ENOENT
	}

	if _, ok := ch.Node().(*mutableLink); !ok {
		return fuse.EPERM
	}

	n.Inode().RmChild(name)
	return fuse.OK
}

type blobNode struct {
	gitNode
	mode int
	size int64
}

type linkNode struct {
	gitNode
	target []byte
}

func (n *linkNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	out.Mode = fuse.S_IFLNK
	return fuse.OK
}

func (n *linkNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	return n.target, fuse.OK
}

func (n *blobNode) Open(flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}

	f, err := os.Open(filepath.Join(n.fs.dir, n.id.String()))
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	return nodefs.NewLoopbackFile(f), fuse.OK
}

func (n *blobNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	out.Mode = uint32(n.mode)
	out.Size = uint64(n.size)
	return fuse.OK
}

func (t *treeFS) newLinkNode(id *git.Oid) (nodefs.Node, error) {
	n := &linkNode{
		gitNode: gitNode{
			fs:   t,
			id:   id.Copy(),
			Node: nodefs.NewDefaultNode(),
		},
	}

	blob, err := t.repo.LookupBlob(id)
	if err != nil {
		return nil, err
	}
	defer blob.Free()
	n.target = append([]byte{}, blob.Contents()...)
	return n, nil
}

func (t *treeFS) newBlobNode(id *git.Oid, mode int) (nodefs.Node, error) {
	n := &blobNode{
		gitNode: gitNode{
			fs:   t,
			id:   id.Copy(),
			Node: nodefs.NewDefaultNode(),
		},
	}

	p := filepath.Join(t.dir, id.String())
	if fi, err := os.Lstat(p); os.IsNotExist(err) {
		blob, err := t.repo.LookupBlob(id)
		if err != nil {
			return nil, err
		}
		defer blob.Free()
		n.size = blob.Size()

		// TODO - atomic, use content store to share content.
		if err := ioutil.WriteFile(p, blob.Contents(), 0644); err != nil {
			return nil, err
		}
	} else {
		n.size = fi.Size()
	}

	n.mode = mode
	return n, nil
}

func (t *treeFS) newDirNode(id *git.Oid) nodefs.Node {
	n := &dirNode{
		gitNode: gitNode{
			fs:   t,
			id:   id.Copy(),
			Node: nodefs.NewDefaultNode(),
		},
	}
	return n
}

func (t *treeFS) recurse(tree *git.Tree, n *dirNode) error {
	for i := uint64(0); ; i++ {
		e := tree.EntryByIndex(i)
		if e == nil {
			break
		}
		isdir := e.Filemode&syscall.S_IFDIR != 0
		var chNode nodefs.Node
		if isdir {
			chNode = t.newDirNode(e.Id)
		} else if e.Filemode&^07777 == syscall.S_IFLNK {
			l, err := t.newLinkNode(e.Id)
			if err != nil {
				return err
			}
			chNode = l
		} else if e.Filemode&^07777 == syscall.S_IFREG {
			b, err := t.newBlobNode(e.Id, e.Filemode)
			if err != nil {
				return err
			}
			chNode = b
		} else {
			panic(e)
		}
		n.Inode().NewChild(e.Name, isdir, chNode)

		if isdir {
			tree, err := t.repo.LookupTree(chNode.(*dirNode).id)
			if err != nil {
				return err
			}

			if err := t.recurse(tree, chNode.(*dirNode)); err != nil {
				return nil
			}
		}
	}
	return nil
}
