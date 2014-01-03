package fs

import (
	"log"
	"syscall"

	git "github.com/libgit2/git2go"

	"github.com/hanwen/go-fuse/fuse/nodefs"
//	"github.com/hanwen/go-fuse/fuse"
)

type treeFS struct {
	nodefs.FileSystem
	repo *git.Repository
	root *dirNode
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

	t := &treeFS{
		repo: repo,
		FileSystem: nodefs.NewDefaultFileSystem(),
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
	id *git.Oid
	nodefs.Node
}

type dirNode struct {
	gitNode
}

type blobNode struct {
	gitNode
}

func (t *treeFS) newBlobNode(id *git.Oid) *blobNode {
	n := &blobNode{
		gitNode: gitNode{
			id: id.Copy(),
			Node: nodefs.NewDefaultNode(),
		},
	}
	return n
}

func (t *treeFS) newDirNode(id *git.Oid) *dirNode {
	n := &dirNode{
		gitNode: gitNode{
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
			chNode = t.newDirNode(e.Id)
		} else {
			chNode = t.newBlobNode(e.Id)
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

type GitNode struct {
	nodefs.Node

}
