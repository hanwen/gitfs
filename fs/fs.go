package fs

import (
	"io/ioutil"
	"path/filepath"
	"syscall"
	"os"

	git "github.com/libgit2/git2go"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse"
)

type treeFS struct {
	nodefs.FileSystem
	repo *git.Repository
	root *dirNode
	dir string
}

func (t *treeFS) Root() nodefs.Node {
	return t.root
}

func NewTreeFS(repo *git.Repository, name string) (nodefs.FileSystem, error) {
	ref, err := repo.LookupReference(name)
	if err != nil {
		return nil, err
	}

	ref, err = ref.Resolve()
	if err != nil {
		return nil, err
	}

	commit, err := repo.LookupCommit(ref.Target())
	if err != nil {
		return nil, err
	}

	dir, err := ioutil.TempDir("", "gitfs")
	if err != nil {
		return nil, err
	}
	
	t := &treeFS{
		repo: repo,
		FileSystem: nodefs.NewDefaultFileSystem(),
		dir: dir,
	}
	t.root = t.newDirNode(commit.TreeId())
	return t, nil
}

func (t *treeFS) OnMount(conn *nodefs.FileSystemConnector) {
	tree, err := t.repo.LookupTree(t.root.id)
	if err != nil {
		panic(err)
	}

	if t.root.Inode() == nil {
		panic("nil?")
	}
	t.recurse(tree, t.root)
	if err != nil {
		panic(err)
	}
}

type gitNode struct {
	fs *treeFS
	id *git.Oid
	nodefs.Node
}

type dirNode struct {
	gitNode
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
	if flags & fuse.O_ANYWRITE != 0 {
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

func (t *treeFS) newLinkNode(id *git.Oid) (*linkNode, error) {
	n := &linkNode{
		gitNode: gitNode{
			fs: t,
			id: id.Copy(),
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

func (t *treeFS) newBlobNode(id *git.Oid) (*blobNode, error) {
	n := &blobNode{
		gitNode: gitNode{
			fs: t,
			id: id.Copy(),
			Node: nodefs.NewDefaultNode(),
		},
	}
	
	blob, err := t.repo.LookupBlob(id)
	if err != nil {
		return nil, err
	}
	defer blob.Free()
	n.size = blob.Size()
	p := filepath.Join(t.dir, id.String())
	if _, err := os.Lstat(p); os.IsNotExist(err) {
		// TODO - atomic, use content store to share content.
		err := ioutil.WriteFile(p, blob.Contents(), 0644)
		if err != nil {
			return nil, err
		}
	}
	
	return n, nil
}

func (t *treeFS) newDirNode(id *git.Oid) *dirNode {
	n := &dirNode{
		gitNode: gitNode{
			fs: t,
			id: id.Copy(),
			Node: nodefs.NewDefaultNode(),
		},
	}
	return n
}

func (t *treeFS) recurse(tree *git.Tree, n *dirNode) error {
	i := 0
	for {
		e := tree.EntryByIndex(uint64(i))
		if e == nil {
			break
		}
		isdir := e.Filemode & syscall.S_IFDIR != 0
		var chNode nodefs.Node
		if isdir {
			d := t.newDirNode(e.Id)
			chNode = d
		} else if e.Filemode &^ 07777 == syscall.S_IFLNK {
			l, err := t.newLinkNode(e.Id)
			if err != nil {
				return err
			}
			chNode = l
		} else if e.Filemode &^ 07777 == syscall.S_IFREG {
			b, err := t.newBlobNode(e.Id)
			if err != nil {
				return err
			}
			b.mode = e.Filemode
			chNode = b
		} else {
			panic(e)
		}
		ch := n.Inode().New(isdir, chNode)

		n.Inode().AddChild(e.Name, ch)
		i++

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

