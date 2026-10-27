package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Called under a.mu: select only a current history entry, never a caller path.
func (a *appServer) fileActionDocument(id string) (Document, string, error) {
	if a.job.Busy {
		return Document{}, "", errors.New("Wait for the current job before changing or copying files")
	}
	if id == "" || filepath.Base(id) != id || !strings.HasPrefix(id, "scan-") {
		return Document{}, "", errors.New("Invalid document")
	}
	var doc Document
	for _, d := range a.docs {
		if d.ID == id {
			doc = d
			break
		}
	}
	if doc.ID == "" {
		return doc, "", errors.New("Document no longer exists. Refresh history")
	}
	root, err := filepath.EvalSymlinks(a.settings.Output)
	if err != nil {
		return doc, "", err
	}
	dir := filepath.Join(root, id)
	st, err := os.Lstat(dir)
	if err != nil {
		return doc, "", err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return doc, "", errors.New("Invalid document folder")
	}
	return doc, dir, nil
}

func (a *appServer) performFileAction(action string, paths []string) error {
	if a.fileAction != nil {
		return a.fileAction(action, paths)
	}
	if a.nativeHelper == "" {
		return errors.New("Open the installed Mac app to copy files or use Trash")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	args := append([]string{"--" + action}, paths...)
	output, err := exec.CommandContext(ctx, a.nativeHelper, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Mac file operation failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// Validate the whole selection before touching the clipboard or any folder.
func (a *appServer) fileActionSelection(ids []string) ([]Document, []string, error) {
	if len(ids) == 0 || len(ids) > 200 {
		return nil, nil, errors.New("Select between 1 and 200 documents")
	}
	docs := []Document{}
	dirs := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		doc, dir, err := a.fileActionDocument(id)
		if err != nil {
			return nil, nil, err
		}
		docs = append(docs, doc)
		dirs = append(dirs, dir)
	}
	return docs, dirs, nil
}
func (a *appServer) copyDocument(id string) error { return a.copyDocuments([]string{id}) }
func (a *appServer) copyDocuments(ids []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	docs, dirs, err := a.fileActionSelection(ids)
	if err != nil {
		return err
	}
	paths := []string{}
	for i, doc := range docs {
		names := doc.Pages
		if doc.PDF != "" {
			names = []string{doc.PDF}
		}
		if len(names) == 0 {
			return errors.New("This document has no files to copy")
		}
		for _, name := range names {
			if name == "" || filepath.Base(name) != name {
				return errors.New("Invalid document file")
			}
			path := filepath.Join(dirs[i], name)
			st, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if !st.Mode().IsRegular() {
				return errors.New("Invalid document file")
			}
			paths = append(paths, path)
		}
	}
	// One write publishes the full batch; per-document writes would replace it.
	return a.performFileAction("copy-files", paths)
}

type TrashResult struct {
	Moved  []string          `json:"moved"`
	Failed map[string]string `json:"failed"`
}

func (a *appServer) trashDocument(id string, confirmed bool) error {
	result, err := a.trashDocuments([]string{id}, confirmed)
	if err != nil {
		return err
	}
	if message := result.Failed[id]; message != "" {
		return errors.New(message)
	}
	return nil
}
func (a *appServer) trashDocuments(ids []string, confirmed bool) (TrashResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := TrashResult{Moved: []string{}, Failed: map[string]string{}}
	if !confirmed {
		return result, errors.New("Confirm moving these documents to Trash first")
	}
	docs, dirs, err := a.fileActionSelection(ids)
	if err != nil {
		return result, err
	}
	for i, doc := range docs {
		if err := a.performFileAction("trash-folder", []string{dirs[i]}); err != nil {
			result.Failed[doc.ID] = err.Error()
		} else {
			result.Moved = append(result.Moved, doc.ID)
		}
	}
	// A batch can partially succeed. Return exact outcomes and retain failed files.
	a.docs = listDocuments(a.settings.Output)
	return result, nil
}
