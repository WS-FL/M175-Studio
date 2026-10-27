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

func (a *appServer) copyDocument(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	doc, dir, err := a.fileActionDocument(id)
	if err != nil {
		return err
	}
	names := doc.Pages
	if doc.PDF != "" {
		names = []string{doc.PDF}
	}
	if len(names) == 0 {
		return errors.New("This document has no files to copy")
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" || filepath.Base(name) != name {
			return errors.New("Invalid document file")
		}
		path := filepath.Join(dir, name)
		st, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return errors.New("Invalid document file")
		}
		paths = append(paths, path)
	}
	return a.performFileAction("copy-files", paths)
}

func (a *appServer) trashDocument(id string, confirmed bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !confirmed {
		return errors.New("Confirm moving this document to Trash first")
	}
	_, dir, err := a.fileActionDocument(id)
	if err != nil {
		return err
	}
	if err = a.performFileAction("trash-folder", []string{dir}); err != nil {
		return err
	}
	a.docs = listDocuments(a.settings.Output)
	return nil
}
