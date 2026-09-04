// Package git reads tags and tagged file contents from a local repository.
//
// It reads and never writes, and it never opens a socket. The go-git
// sub-packages it imports are the object model and the on-disk storage; the
// top-level go-git/v5 package is deliberately not among them, because that
// package registers the HTTP, SSH, git and file transports in an init and Go
// links per package, so importing it for PlainOpen alone would link the crypto
// and SSH stack into a binary that never speaks to a remote.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"golang.org/x/mod/semver"
)

// Tag is a tag reference and the object it names, which is a commit for a
// lightweight tag and a tag object for an annotated one.
type Tag struct {
	// Name is the tag without its refs/tags/ prefix, as it was written.
	Name string
	// Hash is what the reference points at, not necessarily a commit.
	Hash plumbing.Hash
}

// Version is the semver version the tag names, canonical and with its leading
// "v", or empty for a tag naming no version.
//
// It is what two tags are compared by, so a repository tagging 1.2.3 is read
// the same as one tagging v1.2.3.
func (t Tag) Version() string {
	v := canonical(t.Name)
	if !semver.IsValid(v) {
		return ""
	}
	return semver.Canonical(v)
}

// Repo is a read-only handle on a local repository.
type Repo struct {
	store *filesystem.Storage
	root  string
}

// Root is the working tree's top directory, which is what a path inside a tag's
// tree is relative to.
func (r *Repo) Root() string { return r.root }

// Open returns a handle on the repository containing dir, searching dir and
// then each parent for a .git.
//
// A .git file is followed to the directory it names, and a linked worktree's
// commondir is followed from there: tags and objects live in the common
// directory, and the worktree-local gitdir carries neither.
func Open(dir string) (*Repo, error) {
	root, dot, err := discover(dir)
	if err != nil {
		return nil, err
	}
	return &Repo{
		store: filesystem.NewStorage(osfs.New(dot), cache.NewObjectLRUDefault()),
		root:  root,
	}, nil
}

// Shallow reports whether the repository was cloned to a depth, which is what
// a checkout that fetched no tags looks like from inside: the tags exist and
// the clone does not carry them.
func (r *Repo) Shallow() (bool, error) {
	hashes, err := r.store.Shallow()
	if err != nil {
		return false, err
	}
	return len(hashes) > 0, nil
}

// Tags returns every tag the repository carries, annotated and lightweight
// alike, in the order the reference store yields them.
//
// It iterates references rather than tag objects, which is what makes it cover
// both: a lightweight tag has no object of its own to be found.
func (r *Repo) Tags() ([]Tag, error) {
	iter, err := r.store.IterReferences()
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var out []Tag
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference || !ref.Name().IsTag() {
			return nil
		}
		out = append(out, Tag{Name: ref.Name().Short(), Hash: ref.Hash()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FileAt returns the contents of path in the tree the named tag points at.
//
// It reports an error wrapping os.ErrNotExist when the tag carries no such
// file, which is an ordinary state for a tag cut before the file existed.
func (r *Repo) FileAt(tag, path string) ([]byte, error) {
	tags, err := r.Tags()
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Name != tag {
			continue
		}
		commit, err := r.commit(t.Hash)
		if err != nil {
			return nil, err
		}
		tree, err := commit.Tree()
		if err != nil {
			return nil, err
		}
		f, err := tree.File(path)
		if err != nil {
			return nil, fmt.Errorf("%s at tag %s: %w", path, tag, os.ErrNotExist)
		}
		s, err := f.Contents()
		if err != nil {
			return nil, err
		}
		return []byte(s), nil
	}
	return nil, fmt.Errorf("no tag %s: %w", tag, os.ErrNotExist)
}

// commit resolves what a tag reference points at, following annotated tag
// objects, which may in principle name another tag.
func (r *Repo) commit(h plumbing.Hash) (*object.Commit, error) {
	for range 10 {
		obj, err := r.store.EncodedObject(plumbing.AnyObject, h)
		if err != nil {
			return nil, err
		}
		switch obj.Type() {
		case plumbing.CommitObject:
			return object.GetCommit(r.store, h)
		case plumbing.TagObject:
			t, err := object.GetTag(r.store, h)
			if err != nil {
				return nil, err
			}
			h = t.Target
		default:
			return nil, fmt.Errorf("tag object %s is a %s, not a commit", h, obj.Type())
		}
	}
	return nil, fmt.Errorf("tag object %s does not reach a commit", h)
}

// Newest returns the tag naming the highest semver version, and reports false
// when none of them names a version at all.
//
// Tags that are not versions are ignored rather than refused: a repository is
// free to carry tags this tool has no opinion about.
func Newest(tags []Tag) (Tag, bool) {
	versions := Versions(tags)
	if len(versions) == 0 {
		return Tag{}, false
	}
	return versions[0], true
}

// Versions returns the tags naming a semver version, highest first.
func Versions(tags []Tag) []Tag {
	out := make([]Tag, 0, len(tags))
	for _, t := range tags {
		if t.Version() != "" {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return semver.Compare(out[i].Version(), out[j].Version()) > 0
	})
	return out
}

// canonical adds the "v" that golang.org/x/mod/semver requires, so a repository
// tagging 1.2.3 is read the same as one tagging v1.2.3.
func canonical(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if v[0] != 'v' && v[0] != 'V' {
		return "v" + v
	}
	return "v" + v[1:]
}

// discover searches dir and each of its parents for a repository, returning the
// working tree's top directory and the directory holding its refs and objects.
func discover(dir string) (root, dot string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	for {
		candidate := filepath.Join(abs, ".git")
		info, err := os.Stat(candidate)
		if err == nil {
			if !info.IsDir() {
				if candidate, err = gitdir(candidate); err != nil {
					return "", "", err
				}
			}
			dot, err := common(candidate)
			return abs, dot, err
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", "", fmt.Errorf("no git repository at or above %s", dir)
		}
		abs = parent
	}
}

// gitdir reads the "gitdir: <path>" a .git file carries in a linked worktree or
// a submodule.
func gitdir(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", fmt.Errorf("%s names no gitdir", path)
	}
	target := strings.TrimSpace(rest)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), nil
}

// common returns the directory a linked worktree's commondir names, or dot
// itself where there is no such file.
//
// A worktree's own gitdir carries its HEAD and index and neither the refs nor
// the objects, so reading tags from it would find none.
func common(dot string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dot, "commondir"))
	if err != nil {
		return dot, nil //nolint:nilerr // absent commondir means dot is the common directory
	}
	target := strings.TrimSpace(string(b))
	if target == "" {
		return dot, nil
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dot, target)
	}
	return filepath.Clean(target), nil
}
