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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"sarak.sh/mikluko/changelog/v2/pkg/semver"
)

// Tag is a tag reference and the object it names, which is a commit for a
// lightweight tag and a tag object for an annotated one.
type Tag struct {
	// Name is the tag without its refs/tags/ prefix, as it was written.
	Name string
	// Hash is what the reference points at, not necessarily a commit.
	Hash plumbing.Hash
}

// Semver is the version the tag names, and whether it names one at all.
//
// A tag namespace is laxer than the specification: it carries a leading "v",
// and the moving "v1" a release workflow keeps reads as the version it points
// at, which is what leaves it a candidate for the reference tag.
func (t Tag) Semver() (semver.Version, bool) {
	v, err := semver.ParseTag(t.Name)
	return v, err == nil
}

// Version is the semver version the tag names, canonical and with its leading
// "v", or empty for a tag naming no version.
//
// It is what two tags are compared by, so a repository tagging 1.2.3 is read
// the same as one tagging v1.2.3.
func (t Tag) Version() string {
	v, ok := t.Semver()
	if !ok {
		return ""
	}
	return v.Canonical().Tag()
}

// TagDay is the calendar day the tag was cut, as YYYY-MM-DD.
//
// Which date that is depends on the kind of tag, and a repository uses one kind
// or the other rather than both: an annotated tag is an object carrying a
// tagger date of its own, and a lightweight tag is a bare reference with no
// date anywhere but on the commit it names. Assuming either kind alone is wrong
// on half the repositories in reach.
//
// The day is taken in the tag's own timezone and not in UTC. A changelog date
// is a bare calendar day, and what it means is the day the human cut the
// release; normalizing a tag cut at 23:30-07:00 to UTC moves it to the next day
// and reports a correct entry as wrong.
func (r *Repo) TagDay(t Tag) (string, error) {
	when, err := r.tagTime(t.Hash)
	if err != nil {
		return "", err
	}
	return when.Format("2006-01-02"), nil
}

// tagTime returns the moment a tag names, following an annotated tag to its own
// tagger date and a lightweight one to the committer date of the commit it
// points at.
func (r *Repo) tagTime(h plumbing.Hash) (time.Time, error) {
	obj, err := r.store.EncodedObject(plumbing.AnyObject, h)
	if err != nil {
		return time.Time{}, err
	}
	if obj.Type() == plumbing.TagObject {
		t, err := object.GetTag(r.store, h)
		if err != nil {
			return time.Time{}, err
		}
		return t.Tagger.When, nil
	}
	return r.commitTime(h)
}

// commitTime is the committer date of the commit h names, in the offset the
// commit records. It is the one place a lightweight tag's date is read, for a
// tag that exists and for one that has yet to be cut.
func (r *Repo) commitTime(h plumbing.Hash) (time.Time, error) {
	c, err := r.commit(h)
	if err != nil {
		return time.Time{}, err
	}
	return c.Committer.When, nil
}

// NextDay is the calendar day a tag cut on HEAD now would carry, as YYYY-MM-DD,
// and whether that is known before the tag exists.
//
// It depends on the kind of tag, which is read from the repository's own
// version tags, since a repository uses one kind or the other:
//
//   - Where every one is lightweight, the day is the committer date of HEAD in
//     the offset HEAD records, which is what TagDay would answer once a
//     lightweight tag named it. It is read by the same code, so the two cannot
//     part.
//   - Where any is annotated, the day is the tagger date of a tag object not
//     yet written, which is the moment it is cut in the tagger's zone: neither
//     is known beforehand, and the answer is not known.
//   - Where there is no version tag, the kind is not known either, and neither
//     is the day.
//
// An unreadable tag or HEAD is an error, which a caller reads as not known.
func (r *Repo) NextDay(tags []Tag) (string, bool, error) {
	versions := Versions(tags)
	if len(versions) == 0 {
		return "", false, nil
	}
	for _, t := range versions {
		obj, err := r.store.EncodedObject(plumbing.AnyObject, t.Hash)
		if err != nil {
			return "", false, err
		}
		if obj.Type() == plumbing.TagObject {
			return "", false, nil
		}
	}
	head, err := r.head()
	if err != nil {
		return "", false, err
	}
	when, err := r.commitTime(head)
	if err != nil {
		return "", false, err
	}
	return when.Format("2006-01-02"), true, nil
}

// Repo is a read-only handle on a local repository.
type Repo struct {
	// store holds the refs and objects, which a linked worktree shares with the
	// checkout it was added from.
	store *filesystem.Storage
	// local holds HEAD, which is the worktree's own: the same directory as store
	// in an ordinary checkout, and the worktree's private gitdir in a linked one.
	local *filesystem.Storage
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
// directory, and the worktree-local gitdir carries neither. HEAD is the other
// way round: it is read from the worktree-local gitdir, because the common
// directory's HEAD is the main checkout's, and the commits a linked worktree
// reaches are its own.
//
// A git directory that cannot be read is an error, and not a repository whose
// tags happen to be absent. The caller cannot tell those apart afterwards: an
// unreadable store yields no references, which is exactly what a repository
// before its first release yields.
func Open(dir string) (*Repo, error) {
	root, own, dot, err := discover(dir)
	if err != nil {
		return nil, err
	}
	r := &Repo{
		store: filesystem.NewStorage(osfs.New(dot), cache.NewObjectLRUDefault()),
		root:  root,
	}
	r.local = r.store
	if own != dot {
		r.local = filesystem.NewStorage(osfs.New(own), cache.NewObjectLRUDefault())
	}
	return r, nil
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

// HeadFile returns the contents of path in the tree HEAD names, as FileAt does
// for a tag, and reports an error wrapping os.ErrNotExist where HEAD carries no
// such file.
func (r *Repo) HeadFile(path string) ([]byte, error) {
	h, err := r.head()
	if err != nil {
		return nil, err
	}
	commit, err := r.commit(h)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	f, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("%s at HEAD: %w", path, os.ErrNotExist)
	}
	s, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
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

// Eligible says which tags may serve as the reference tag.
type Eligible int

const (
	// Final admits only tags naming a version with no pre-release part.
	Final Eligible = iota
	// All admits every tag naming a version.
	All
)

// String is the spelling ParseEligible reads.
func (e Eligible) String() string {
	if e == All {
		return "all"
	}
	return "final"
}

// ParseEligible reads the -reference-tags value, and reports an error naming
// both spellings for anything else.
func ParseEligible(s string) (Eligible, error) {
	switch strings.TrimSpace(s) {
	case "final":
		return Final, nil
	case "all":
		return All, nil
	}
	return Final, fmt.Errorf("-reference-tags %q is not one of final, all", s)
}

// Reference returns the newest eligible version tag reachable from HEAD, which
// is what git describe --tags names, and reports false where the checkout
// reaches no such tag.
//
// Reachability takes no option, because a tag on a branch this checkout is not
// on is never the right baseline. It is what lets a maintained support line
// read its own newest tag: the other line's tag is simply unreachable.
//
// Tags that are not versions are ignored rather than refused, and so is a tag
// whose commit the checkout does not carry: a repository is free to hold tags
// this tool has no opinion about, and a baseline that cannot be read is not one.
func (r *Repo) Reference(tags []Tag, admit Eligible) (Tag, bool, error) {
	candidates := Versions(tags)
	if admit == Final {
		candidates = final(candidates)
	}
	if len(candidates) == 0 {
		return Tag{}, false, nil
	}

	rank := r.rank(candidates)

	head, err := r.head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return Tag{}, false, nil
		}
		return Tag{}, false, err
	}
	if _, err := object.GetCommit(r.store, head); err != nil {
		return Tag{}, false, fmt.Errorf("HEAD at %s: %w", head, err)
	}

	best := r.best(head, rank, len(candidates))
	if best == len(candidates) {
		return Tag{}, false, nil
	}
	return candidates[best], true, nil
}

// head returns the commit HEAD names, which is the worktree's own HEAD and not
// the main checkout's, and reports plumbing.ErrReferenceNotFound where it names
// a branch not yet born.
//
// The file is read from the worktree's gitdir and the branch it names is
// resolved in the common directory, where every worktree's branches live.
func (r *Repo) head() (plumbing.Hash, error) {
	ref, err := r.local.Reference(plumbing.HEAD)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if ref.Type() == plumbing.SymbolicReference {
		if ref, err = storer.ResolveReference(r.store, ref.Target()); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	return ref.Hash(), nil
}

// rank maps the commit each candidate names to its index, candidates being
// highest first, so where two tags name one commit the higher version wins. A
// candidate whose commit cannot be read is left out.
//
// Resolving each candidate up front makes the walk one map lookup per commit
// rather than one ancestry query per tag.
func (r *Repo) rank(candidates []Tag) map[plumbing.Hash]int {
	rank := make(map[plumbing.Hash]int, len(candidates))
	for i, t := range candidates {
		commit, err := r.commit(t.Hash)
		if err != nil {
			continue
		}
		if _, ok := rank[commit.Hash]; !ok {
			rank[commit.Hash] = i
		}
	}
	return rank
}

// best returns the lowest rank of any commit reachable from head, or none
// where no ranked commit is reachable.
//
// The walk stops the moment rank 0 is reached, so a checkout standing at or
// just past its newest tag pays for the commits in between and no more.
func (r *Repo) best(head plumbing.Hash, rank map[plumbing.Hash]int, none int) int {
	best := none
	seen := make(map[plumbing.Hash]bool)
	queue := []plumbing.Hash{head}
	for len(queue) > 0 && best != 0 {
		h := queue[0]
		queue = queue[1:]
		if seen[h] {
			continue
		}
		seen[h] = true
		if i, ok := rank[h]; ok && i < best {
			best = i
		}
		commit, err := object.GetCommit(r.store, h)
		if err != nil {
			// A shallow checkout's boundary commit names parents it does not
			// carry, and there is nothing below it to reach.
			continue
		}
		queue = append(queue, commit.ParentHashes...)
	}
	return best
}

// final drops the tags naming a pre-release version, which is what costs a
// repository that never tags one nothing.
func final(tags []Tag) []Tag {
	out := make([]Tag, 0, len(tags))
	for _, t := range tags {
		// Every tag here came from Versions, so it names one.
		if v, ok := t.Semver(); ok && !v.Prerelease() {
			out = append(out, t)
		}
	}
	return out
}

// Versions returns the tags naming a semver version, highest first.
//
// Two tags can name one version: a repository following the GitHub Actions
// convention carries v1 beside v1.0.0, and both read as v1.0.0. They are ordered
// by how fully each spells that version, so the reference tag is the immutable
// v1.0.0 rather than the v1 that moves to the next release under whoever checked
// it out.
func Versions(tags []Tag) []Tag {
	out := make([]Tag, 0, len(tags))
	for _, t := range tags {
		if _, ok := t.Semver(); ok {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := out[i].Semver()
		b, _ := out[j].Semver()
		if c := semver.Compare(a, b); c != 0 {
			return c > 0
		}
		return semver.TagComponents(out[i].Name) > semver.TagComponents(out[j].Name)
	})
	return out
}

// discover searches dir and each of its parents for a repository, returning the
// working tree's top directory, the directory holding its own HEAD, and the
// directory holding its refs and objects. The last two are one directory except
// in a linked worktree.
func discover(dir string) (root, own, dot string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", "", err
	}
	for {
		candidate := filepath.Join(abs, ".git")
		if info, err := os.Stat(candidate); err == nil {
			own, dot, err := store(candidate, info.IsDir())
			if err != nil {
				return "", "", "", err
			}
			return abs, own, dot, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", "", "", fmt.Errorf("no git repository at or above %s", dir)
		}
		abs = parent
	}
}

// store returns the directory holding the HEAD of the checkout whose .git is at
// candidate, a directory or a file naming one, and the directory holding the
// tags and objects it shares with the others.
func store(candidate string, isDir bool) (own, dot string, err error) {
	if !isDir {
		named := candidate
		if candidate, err = gitdir(named); err != nil {
			return "", "", err
		}
		if err := readable(candidate, named); err != nil {
			return "", "", err
		}
	}
	dot, err = common(candidate)
	if err != nil {
		return "", "", err
	}
	if err := readable(dot, candidate); err != nil {
		return "", "", err
	}
	return candidate, dot, nil
}

// readable reports that dir, which named it, is not a directory this process
// can read.
//
// It is what separates a repository that cannot be read from one that is not
// there. A .git file names an absolute path, and nothing guarantees that path
// resolves here: a linked worktree copied away from its parent, a submodule
// without the superproject, a container mounting the working tree and not the
// repository. Left unchecked, the storage layer opens on the missing directory
// and yields an empty reference set, which reads as a repository that has never
// been tagged.
func readable(dir, named string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%s names the git directory %s, which cannot be read: %w", named, dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s names the git directory %s, which is not a directory", named, dir)
	}
	return nil
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

// Snapshot is the tree of one commit, read by paths relative to Root.
type Snapshot struct {
	tree *object.Tree
}

// MergeBase returns the tree of the merge base of HEAD and base.
//
// Base is a branch as a pull request names its target, as in main, and is
// looked up as origin's remote-tracking branch before a local one; a full
// reference name or a commit hash is taken as it is. A checkout too shallow to
// reach the merge base is an error.
func (r *Repo) MergeBase(base string) (Snapshot, error) {
	head, err := r.head()
	if err != nil {
		return Snapshot{}, fmt.Errorf("HEAD: %w", err)
	}
	ours, err := object.GetCommit(r.store, head)
	if err != nil {
		return Snapshot{}, fmt.Errorf("HEAD at %s: %w", head, err)
	}
	h, err := r.resolve(base)
	if err != nil {
		return Snapshot{}, err
	}
	theirs, err := object.GetCommit(r.store, h)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%s at %s: %w", base, h, err)
	}
	bases, err := ours.MergeBase(theirs)
	if err != nil {
		return Snapshot{}, fmt.Errorf("the merge base of HEAD and %s: %w", base, err)
	}
	if len(bases) == 0 {
		return Snapshot{}, fmt.Errorf("HEAD and %s share no history this checkout carries", base)
	}
	tree, err := bases[0].Tree()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{tree: tree}, nil
}

// Files returns the names of the files directly in dir. A dir the snapshot
// does not carry holds no files.
func (s Snapshot) Files(dir string) ([]string, error) {
	sub, err := s.tree.Tree(filepath.ToSlash(filepath.Clean(dir)))
	if errors.Is(err, object.ErrDirectoryNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range sub.Entries {
		if e.Mode.IsFile() {
			out = append(out, e.Name)
		}
	}
	return out, nil
}

// File returns the contents of path, wrapping os.ErrNotExist where the
// snapshot carries no such file.
func (s Snapshot) File(path string) ([]byte, error) {
	f, err := s.tree.File(filepath.ToSlash(filepath.Clean(path)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, os.ErrNotExist)
	}
	c, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return []byte(c), nil
}

// resolve finds the commit base names, trying origin's remote-tracking branch,
// a local branch and a full reference name in that order, then a commit hash.
func (r *Repo) resolve(base string) (plumbing.Hash, error) {
	for _, name := range []string{"refs/remotes/origin/" + base, "refs/heads/" + base, base} {
		ref, err := storer.ResolveReference(r.store, plumbing.ReferenceName(name))
		if err == nil {
			return ref.Hash(), nil
		}
	}
	if plumbing.IsHash(base) {
		return plumbing.NewHash(base), nil
	}
	return plumbing.ZeroHash, fmt.Errorf("no branch, reference or commit %s", base)
}
