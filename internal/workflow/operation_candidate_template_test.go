package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// rocS8FixtureTemplate is a frozen snapshot of one rocS8BuildFixture result.
// Building that fixture costs seconds of Git and reconcile work and about
// sixty ROCS8 rows each need a private one, so each test process builds it
// once per kind and gives every row a copy (TW-D16). A copy has the
// snapshot's bytes, permission bits and file modification times, except:
//   - the two links Git writes with the repository's absolute path for each
//     linked worktree (.git/worktrees/<id>/gitdir and <worktree>/.git) name
//     the copy's root instead; and
//   - every index is refreshed, because the copy's files have new inodes and
//     change times, which Git would otherwise notice during the row itself.
type rocS8FixtureTemplate struct {
	once      sync.Once
	ready     bool
	dir       string   // snapshot root; TestMain removes it
	gitRoot   string   // Git's spelling of the build root inside the links
	roots     []string // every spelling of the build and snapshot roots
	worktrees []rocS8Worktree
	fixture   rocS8Fixture // store is nil
}

type rocS8Worktree struct{ id, rel string }

var rocS8Templates [2]rocS8FixtureTemplate // index 1: creation fixtures

// rocS8TemplateSupported keeps Windows on fresh builds: its native S8 step
// has ample budget, and this copier is not verified against Git for
// Windows' path spellings (TW-D17).
func rocS8TemplateSupported() bool { return runtime.GOOS != "windows" }

// rocS8RemoveTemplates runs from TestMain after every test has finished.
func rocS8RemoveTemplates() {
	for i := range rocS8Templates {
		if dir := rocS8Templates[i].dir; dir != "" {
			_ = os.RemoveAll(dir)
		}
	}
}

func rocS8TemplateFor(t *testing.T, creation bool) *rocS8FixtureTemplate {
	t.Helper()
	tmpl := &rocS8Templates[0]
	if creation {
		tmpl = &rocS8Templates[1]
	}
	tmpl.once.Do(func() {
		built := rocS8BuildFixture(t, creation)
		dir, err := os.MkdirTemp("", "tpatch-rocs8-template-")
		if err != nil {
			t.Fatal(err)
		}
		tmpl.dir = dir
		tmpl.gitRoot = git(t, built.s.Root, "rev-parse", "--show-toplevel")
		tmpl.roots = rocS8RootSpellings(t, built.s.Root, dir, tmpl.gitRoot)
		rocS8CopyTree(t, built.s.Root, dir)
		tmpl.worktrees = rocS8ParseWorktrees(t, dir, tmpl.gitRoot)
		tmpl.fixture = *rocS8CloneFixture(t, built, nil)
		tmpl.ready = true
	})
	if !tmpl.ready {
		t.Fatal("the ROCS8 fixture template failed to build in an earlier test of this process")
	}
	return tmpl
}

func (tmpl *rocS8FixtureTemplate) instantiate(t *testing.T) *rocS8Fixture {
	t.Helper()
	root := t.TempDir()
	rocS8CopyTree(t, tmpl.dir, root)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rocS8RelinkWorktrees(t, root, tmpl.gitRoot, git(t, root, "rev-parse", "--show-toplevel"))
	if leaks := rocS8FilesNaming(t, root, tmpl.roots); len(leaks) != 0 {
		t.Fatalf("ROCS8 fixture copy still names the template: %v", leaks)
	}
	git(t, root, "update-index", "-q", "--refresh")
	for _, worktree := range tmpl.worktrees {
		git(t, filepath.Join(root, filepath.FromSlash(worktree.rel)), "update-index", "-q", "--refresh")
	}
	return rocS8CloneFixture(t, &tmpl.fixture, s)
}

func rocS8RootSpellings(t *testing.T, roots ...string) []string {
	t.Helper()
	var out []string
	for _, root := range roots {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, spelling := range []string{root, resolved, filepath.ToSlash(root), filepath.ToSlash(resolved)} {
			if !slices.Contains(out, spelling) {
				out = append(out, spelling)
			}
		}
	}
	return out
}

// rocS8CopyTree copies regular files and directories with their permission
// bits and modification times, and refuses every other entry type.
func rocS8CopyTree(t *testing.T, src, dst string) {
	t.Helper()
	type directory struct {
		path string
		info fs.FileInfo
	}
	var directories []directory
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			if rel != "." {
				if err := os.Mkdir(target, 0o700); err != nil {
					return err
				}
			}
			directories = append(directories, directory{target, info})
		case info.Mode().IsRegular():
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, raw, 0o600); err != nil {
				return err
			}
			if err := os.Chmod(target, info.Mode().Perm()); err != nil {
				return err
			}
			return os.Chtimes(target, info.ModTime(), info.ModTime())
		default:
			return fmt.Errorf("%s: the ROCS8 template copies only regular files and directories, not %v", rel, info.Mode().Type())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Children before parents: adding an entry changes a directory's
	// modification time, and a read-only directory would refuse it.
	for i := len(directories) - 1; i >= 0; i-- {
		d := directories[i]
		if err := os.Chmod(d.path, d.info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(d.path, d.info.ModTime(), d.info.ModTime()); err != nil {
			t.Fatal(err)
		}
	}
}

// rocS8ParseWorktrees reads root's linked-worktree registry, whose links
// must name gitRoot exactly as `git worktree add` writes them.
func rocS8ParseWorktrees(t *testing.T, root, gitRoot string) []rocS8Worktree {
	t.Helper()
	registry := filepath.Join(root, ".git", "worktrees")
	entries, err := os.ReadDir(registry)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []rocS8Worktree
	for _, entry := range entries {
		id := entry.Name()
		content := rocS8ReadImage(t, filepath.Join(registry, id, "gitdir")).data
		rel, ok := strings.CutPrefix(content, gitRoot+"/")
		if ok {
			rel, ok = strings.CutSuffix(rel, "/.git\n")
		}
		if !ok || !filepath.IsLocal(filepath.FromSlash(rel)) {
			t.Fatalf("worktree %s backlink %q is not a Git-written link under %q", id, content, gitRoot)
		}
		gitfile := filepath.Join(root, filepath.FromSlash(rel), ".git")
		if got, want := rocS8ReadImage(t, gitfile).data, rocS8Gitfile(gitRoot, id); got != want {
			t.Fatalf("worktree %s gitfile is %q, want %q", id, got, want)
		}
		out = append(out, rocS8Worktree{id, rel})
	}
	return out
}

func rocS8Gitfile(gitRoot, id string) string {
	return "gitdir: " + gitRoot + "/.git/worktrees/" + id + "\n"
}

func rocS8Backlink(gitRoot, rel string) string { return gitRoot + "/" + rel + "/.git\n" }

// rocS8RelinkWorktrees rewrites both links of every linked worktree under
// root from the build root's spelling to to's, keeping mode and mtime.
func rocS8RelinkWorktrees(t *testing.T, root, from, to string) {
	t.Helper()
	for _, worktree := range rocS8ParseWorktrees(t, root, from) {
		rocS8RewriteInPlace(t, filepath.Join(root, filepath.FromSlash(worktree.rel), ".git"), rocS8Gitfile(to, worktree.id))
		rocS8RewriteInPlace(t, filepath.Join(root, ".git", "worktrees", worktree.id, "gitdir"), rocS8Backlink(to, worktree.rel))
	}
}

func rocS8RewriteInPlace(t *testing.T, name, content string) {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(name, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func rocS8FilesNaming(t *testing.T, root string, needles []string) []string {
	t.Helper()
	var hits []string
	for name, image := range rocS8TreeImages(t, root) {
		for _, needle := range needles {
			if strings.Contains(image.data, needle) {
				hits = append(hits, name+" names "+needle)
			}
		}
	}
	sort.Strings(hits)
	return hits
}

// rocS8CloneFixture deep-copies every field except the store, which is s.
func rocS8CloneFixture(t *testing.T, src *rocS8Fixture, s *store.Store) *rocS8Fixture {
	t.Helper()
	inner := *src.rocS2Fixture
	inner.s = s
	inner.invocation.slugs = slices.Clone(inner.invocation.slugs)
	inner.coverage = rocS8DeepCopy(t, inner.coverage)
	inner.event = rocS8DeepCopy(t, inner.event)
	inner.patch, inner.recipe = bytes.Clone(inner.patch), bytes.Clone(inner.recipe)
	out := *src
	out.rocS2Fixture = &inner
	out.artifact = rocS8DeepCopy(t, out.artifact)
	return &out
}

func rocS8DeepCopy[T any](t *testing.T, value T) T {
	t.Helper()
	out := reflect.New(reflect.TypeOf(value)).Elem()
	if err := rocS8DeepCopyValue(out, reflect.ValueOf(value)); err != nil {
		t.Fatal(err)
	}
	return out.Interface().(T)
}

func rocS8DeepCopyValue(dst, src reflect.Value) error {
	switch src.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		dst.Set(src)
	case reflect.Slice:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := range src.Len() {
			if err := rocS8DeepCopyValue(dst.Index(i), src.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Array:
		for i := range src.Len() {
			if err := rocS8DeepCopyValue(dst.Index(i), src.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for iter := src.MapRange(); iter.Next(); {
			key, value := reflect.New(src.Type().Key()).Elem(), reflect.New(src.Type().Elem()).Elem()
			if err := rocS8DeepCopyValue(key, iter.Key()); err != nil {
				return err
			}
			if err := rocS8DeepCopyValue(value, iter.Value()); err != nil {
				return err
			}
			dst.SetMapIndex(key, value)
		}
	case reflect.Pointer:
		if src.IsNil() {
			return nil
		}
		dst.Set(reflect.New(src.Type().Elem()))
		return rocS8DeepCopyValue(dst.Elem(), src.Elem())
	case reflect.Struct:
		for i := range src.NumField() {
			if field := src.Type().Field(i); !field.IsExported() {
				return fmt.Errorf("the ROCS8 template cannot deep-copy unexported field %s.%s", src.Type(), field.Name)
			}
			if err := rocS8DeepCopyValue(dst.Field(i), src.Field(i)); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("the ROCS8 template cannot deep-copy %s", src.Type())
	}
	return nil
}

// rocS8SharedStorage walks a and b together and reports every slice, map
// or pointer whose storage they share. checked counts the references it
// compared, so a caller can refuse a vacuous walk.
func rocS8SharedStorage(a, b reflect.Value, path string) (shared []string, checked int) {
	if a.Type() != b.Type() {
		return []string{path + ": different types"}, 0
	}
	walk := func(x, y reflect.Value, sub string) {
		s, c := rocS8SharedStorage(x, y, sub)
		shared, checked = append(shared, s...), checked+c
	}
	switch a.Kind() {
	case reflect.Slice, reflect.Map:
		if a.Len() > 0 && b.Len() > 0 {
			checked++
			if a.Pointer() == b.Pointer() {
				shared = append(shared, path)
			}
		}
		if a.Kind() == reflect.Map {
			for _, key := range a.MapKeys() {
				if other := b.MapIndex(key); other.IsValid() {
					walk(a.MapIndex(key), other, fmt.Sprintf("%s[%v]", path, key))
				}
			}
			break
		}
		for i := range min(a.Len(), b.Len()) {
			walk(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Array:
		for i := range a.Len() {
			walk(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Pointer:
		if !a.IsNil() && !b.IsNil() {
			checked++
			if a.Pointer() == b.Pointer() {
				shared = append(shared, path)
			}
			walk(a.Elem(), b.Elem(), path)
		}
	case reflect.Struct:
		for i := range a.NumField() {
			walk(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name)
		}
	}
	return shared, checked
}

// rocS8SameFixture compares every field except the store.
func rocS8SameFixture(a, b *rocS8Fixture) bool {
	x, y := *a, *b
	xi, yi := *x.rocS2Fixture, *y.rocS2Fixture
	xi.s, yi.s = nil, nil
	x.rocS2Fixture, y.rocS2Fixture = &xi, &yi
	return reflect.DeepEqual(x, y)
}

type rocS8TreeImage struct {
	mode    fs.FileMode
	modTime time.Time
	data    string
}

func rocS8TreeImages(t *testing.T, root string) map[string]rocS8TreeImage {
	t.Helper()
	out := map[string]rocS8TreeImage{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		image := rocS8TreeImage{mode: info.Mode(), modTime: info.ModTime()}
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			image.data = string(raw)
		}
		out[filepath.ToSlash(rel)] = image
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func rocS8TreeDigest(t *testing.T, root string) string {
	t.Helper()
	images := rocS8TreeImages(t, root)
	names := make([]string, 0, len(images))
	for name := range images {
		names = append(names, name)
	}
	sort.Strings(names)
	sum := sha256.New()
	for _, name := range names {
		image := images[name]
		fmt.Fprintf(sum, "%q %v %d %d %s\n", name, image.mode, image.modTime.UnixNano(), len(image.data), image.data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// rocS8StagedEntries lists one index file's entries without consulting any
// worktree, so a template index and a copy index compare directly.
func rocS8StagedEntries(t *testing.T, root, index string) string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "--stage")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(root, filepath.FromSlash(index)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files %s in %s: %v\n%s", index, root, err, out)
	}
	return string(out)
}

// rocS8CheckCopy reports every way f differs from a faithful, independent
// copy of tmpl.
func rocS8CheckCopy(t *testing.T, tmpl *rocS8FixtureTemplate, f *rocS8Fixture) []string {
	t.Helper()
	var problems []string
	resolved, err := filepath.EvalSymlinks(f.s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range tmpl.roots {
		if resolved == spelling || strings.HasPrefix(resolved, spelling+string(filepath.Separator)) {
			problems = append(problems, "copy root is inside the template or build root: "+resolved)
		}
	}
	if !rocS8SameFixture(f, &tmpl.fixture) {
		problems = append(problems, "in-memory fixture differs from the template")
	}
	shared, checked := rocS8SharedStorage(reflect.ValueOf(*f), reflect.ValueOf(tmpl.fixture), "fixture")
	if checked == 0 {
		problems = append(problems, "storage-sharing walk compared no references")
	}
	for _, path := range shared {
		problems = append(problems, "copy shares storage with the template at "+path)
	}
	if len(tmpl.worktrees) == 0 {
		problems = append(problems, "template has no linked worktree, so relinking is untested")
	}
	gitRoot := git(t, f.s.Root, "rev-parse", "--show-toplevel")
	links := map[string]string{}
	indexes := []string{".git/index"}
	for _, worktree := range tmpl.worktrees {
		links[".git/worktrees/"+worktree.id+"/gitdir"] = rocS8Backlink(gitRoot, worktree.rel)
		links[worktree.rel+"/.git"] = rocS8Gitfile(gitRoot, worktree.id)
		indexes = append(indexes, ".git/worktrees/"+worktree.id+"/index")
		dir := filepath.Join(f.s.Root, filepath.FromSlash(worktree.rel))
		if got, want := git(t, dir, "rev-parse", "--absolute-git-dir"), gitRoot+"/.git/worktrees/"+worktree.id; got != want {
			problems = append(problems, fmt.Sprintf("worktree %s uses Git directory %s, want %s", worktree.rel, got, want))
		}
	}
	want, got := rocS8TreeImages(t, tmpl.dir), rocS8TreeImages(t, f.s.Root)
	names := map[string]bool{}
	for name := range want {
		names[name] = true
	}
	for name := range got {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		w, okW := want[name]
		g, okG := got[name]
		switch {
		case strings.HasPrefix(name[strings.LastIndex(name, "/")+1:], "sharedindex."):
			// A user's core.splitIndex makes a refresh add or rewrite
			// these; staged entries are compared below instead.
		case !okW || !okG:
			problems = append(problems, fmt.Sprintf("%s: present in template %v, in copy %v", name, okW, okG))
		case w.mode != g.mode:
			problems = append(problems, fmt.Sprintf("%s: mode %v, template %v", name, g.mode, w.mode))
		case w.mode.IsDir() || slices.Contains(indexes, name):
			// Index refreshes, and later the rows, change these times and
			// bytes; staged entries are compared below instead.
		case links[name] != "":
			if g.data != links[name] || !g.modTime.Equal(w.modTime) {
				problems = append(problems, fmt.Sprintf("%s: %q is not the relinked template link %q", name, g.data, links[name]))
			}
		case g.data != w.data || !g.modTime.Equal(w.modTime):
			problems = append(problems, fmt.Sprintf("%s: bytes or modification time differ from the template", name))
		}
	}
	for _, index := range indexes {
		if rocS8StagedEntries(t, tmpl.dir, index) != rocS8StagedEntries(t, f.s.Root, index) {
			problems = append(problems, index+": staged entries differ from the template")
		}
	}
	for _, rel := range append([]string{"."}, rocS8WorktreeRels(tmpl)...) {
		dir := filepath.Join(f.s.Root, filepath.FromSlash(rel))
		// diff-files trusts cached stat data and diff reads content, so
		// they agree only while the index is fresh for the copy's files.
		if stat, content := git(t, dir, "diff-files", "--name-only"), git(t, dir, "diff", "--name-only"); stat != content {
			problems = append(problems, fmt.Sprintf("%s: index is stale for the copy (diff-files %q, diff %q)", rel, stat, content))
		}
	}
	return problems
}

func rocS8WorktreeRels(tmpl *rocS8FixtureTemplate) []string {
	var out []string
	for _, worktree := range tmpl.worktrees {
		out = append(out, worktree.rel)
	}
	return out
}

func TestROCS8FixtureTemplateCopiesAreFaithfulAndIndependent(t *testing.T) {
	if !rocS8TemplateSupported() {
		t.Skip("ROCS8 rows build every fixture fresh on Windows (TW-D17)")
	}
	for _, creation := range []bool{false, true} {
		t.Run(fmt.Sprintf("creation-%v", creation), func(t *testing.T) {
			first, second := rocS8NewFixture(t, creation), rocS8NewFixture(t, creation)
			tmpl := rocS8TemplateFor(t, creation)
			for _, f := range []*rocS8Fixture{first, second} {
				if problems := rocS8CheckCopy(t, tmpl, f); len(problems) != 0 {
					t.Fatalf("copy is not faithful and independent:\n%s", strings.Join(problems, "\n"))
				}
			}
			if shared, _ := rocS8SharedStorage(reflect.ValueOf(*first), reflect.ValueOf(*second), "fixture"); len(shared) != 0 {
				t.Fatalf("two copies share storage: %v", shared)
			}
			template, other := rocS8TreeDigest(t, tmpl.dir), rocS8TreeDigest(t, second.s.Root)
			if err := AcceptOperationCandidate(first.s, first.slug); err != nil {
				t.Fatalf("a copy's pending candidate did not accept: %v", err)
			}
			if rocS8TreeDigest(t, tmpl.dir) != template {
				t.Fatal("accepting a copy changed the template")
			}
			if rocS8TreeDigest(t, second.s.Root) != other {
				t.Fatal("accepting a copy changed another copy")
			}
			if err := AcceptOperationCandidate(second.s, second.slug); err != nil {
				t.Fatalf("a second copy did not accept after the first: %v", err)
			}
		})
	}
}

func TestROCS8FixtureTemplateChecksAreSensitive(t *testing.T) {
	if !rocS8TemplateSupported() {
		t.Skip("ROCS8 rows build every fixture fresh on Windows (TW-D17)")
	}
	tmpl := rocS8TemplateFor(t, false)
	deep := rocS8CloneFixture(t, &tmpl.fixture, nil)
	if shared, checked := rocS8SharedStorage(reflect.ValueOf(*deep), reflect.ValueOf(tmpl.fixture), "fixture"); len(shared) != 0 || checked < 4 {
		t.Fatalf("deep clone: shared %v after %d references", shared, checked)
	}
	if !rocS8SameFixture(deep, &tmpl.fixture) {
		t.Fatal("deep clone is not equal to its source")
	}
	inner := *tmpl.fixture.rocS2Fixture
	shallow := tmpl.fixture
	shallow.rocS2Fixture = &inner
	shared, _ := rocS8SharedStorage(reflect.ValueOf(shallow), reflect.ValueOf(tmpl.fixture), "fixture")
	for _, path := range []string{"fixture.rocS2Fixture.invocation.slugs", "fixture.rocS2Fixture.patch", "fixture.rocS2Fixture.recipe"} {
		if !slices.Contains(shared, path) {
			t.Errorf("shallow copy: sharing check missed %s (reported %v)", path, shared)
		}
	}
	deep.patch[0] ^= 1
	if rocS8SameFixture(deep, &tmpl.fixture) {
		t.Error("equality check missed a changed patch byte")
	}
	for _, value := range []any{struct{ hidden int }{}, func() {}} {
		if err := rocS8DeepCopyValue(reflect.New(reflect.TypeOf(value)).Elem(), reflect.ValueOf(value)); err == nil {
			t.Errorf("deep copy accepted %T", value)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("names "+tmpl.gitRoot), 0o644); err != nil {
		t.Fatal(err)
	}
	if hits := rocS8FilesNaming(t, root, tmpl.roots); len(hits) == 0 {
		t.Error("leak scan missed a file naming the build root")
	}
}
