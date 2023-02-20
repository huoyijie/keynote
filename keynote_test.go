package main

import (
	"path/filepath"
	"testing"
)

const (
	keynotesDir = "tests"
)

func TestLoadFolderProps(t *testing.T) {
	conf := filepath.Join(keynotesDir, ".folder.yaml")
	props := loadFolderProps(conf)
	if len(props.Keynote) != 1 || len(props.Docsify) != 1 || len(props.Gitbook) != 1 {
		t.Fatalf("load folder config failed %v", props)
	}

	if kind, found := props.getFileKind("what-is-keynote.md"); !found || !kind.IsKeynote() {
		t.Fatalf("not a keynote %v", props)
	}

	if kind, found := props.getFileKind("what-is-docsify.md"); !found || !kind.IsDocsify() {
		t.Fatalf("not a docsify %v", props)
	}

	if kind, found := props.getFileKind("this-is-a-gitbook"); !found || !kind.IsGitbook() {
		t.Fatalf("not a gitbook %v", props)
	}
}

func TestLoadKeynotes(t *testing.T) {
	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})
	if len(rootFolder.SubFolders) != 1 || len(rootFolder.Files) != 3 {
		t.Fatalf("load keynotes failed %v", rootFolder)
	}

  subFolder := rootFolder.SubFolders[0]
  if len(subFolder.Files) != 1 || len(subFolder.SubFolders) != 0 {
    t.Fatalf("load keynotes failed %v", subFolder)
  }
}
