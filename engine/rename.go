package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func safeTitle(title string) string {
	title = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return ' '
		}
		return r
	}, title)
	title = strings.Trim(strings.TrimSpace(title), ". ")
	if title == "" {
		title = "Untitled scan"
	}
	r := []rune(title)
	if len(r) > 60 {
		title = strings.TrimSpace(string(r[:60]))
	}
	return title
}

// Preserve stable scan-* IDs and page names. Only display metadata and the main
// PDF name change, so legacy histories and all relative JPEG references survive.
func (a *appServer) renameDocument(id, title string) (Document, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.job.Busy {
		return Document{}, errors.New("Wait for the current job before renaming")
	}
	var doc Document
	for _, d := range a.docs {
		if d.ID == id {
			doc = d
			break
		}
	}
	if doc.ID == "" || filepath.Base(id) != id {
		return doc, errors.New("Document no longer exists. Refresh history")
	}
	dir := filepath.Join(a.settings.Output, id)
	st, err := os.Lstat(dir)
	if err != nil {
		return doc, err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return doc, errors.New("Invalid document folder")
	}
	base := safeTitle(title)
	title = base
	for n := 2; ; n++ {
		collision := false
		for _, d := range listDocuments(a.settings.Output) {
			if d.ID != id && strings.EqualFold(d.Title, title) {
				collision = true
				break
			}
		}
		if doc.PDF != "" && !strings.EqualFold(doc.PDF, title+".pdf") {
			if _, e := os.Lstat(filepath.Join(dir, title+".pdf")); !os.IsNotExist(e) {
				collision = true
			}
		}
		if !collision {
			break
		}
		title = fmt.Sprintf("%s %d", base, n)
	}
	oldPDF, newPDF := filepath.Join(dir, doc.PDF), filepath.Join(dir, title+".pdf")
	moved := doc.PDF != "" && doc.PDF != title+".pdf"
	caseOnly := false
	if moved {
		if filepath.Base(doc.PDF) != doc.PDF {
			return doc, errors.New("Invalid PDF path")
		}
		st, e := os.Lstat(oldPDF)
		if e != nil {
			return doc, e
		}
		if !st.Mode().IsRegular() {
			return doc, errors.New("Invalid PDF file")
		}
		// Hard link reserves the new name without replacing any existing file.
		if dest, e := os.Lstat(newPDF); e == nil && os.SameFile(st, dest) {
			caseOnly = true
		}
		if caseOnly {
			e = os.Rename(oldPDF, newPDF)
		} else {
			e = os.Link(oldPDF, newPDF)
		}
		if e != nil {
			return doc, e
		}
	}
	updated := doc
	updated.Title = title
	if doc.PDF != "" {
		updated.PDF = title + ".pdf"
	}
	if err = atomicJSON(filepath.Join(dir, "m175-studio.json"), updated); err != nil {
		if moved {
			if caseOnly {
				_ = os.Rename(newPDF, oldPDF)
			} else {
				_ = os.Remove(newPDF)
			}
		}
		return doc, err
	}
	if moved && !caseOnly {
		if err = os.Remove(oldPDF); err != nil {
			rollback := atomicJSON(filepath.Join(dir, "m175-studio.json"), doc)
			if rollback == nil {
				_ = os.Remove(newPDF)
			}
			return doc, fmt.Errorf("Could not finish PDF rename: %w", err)
		}
	}
	a.docs = listDocuments(a.settings.Output)
	return updated, nil
}
