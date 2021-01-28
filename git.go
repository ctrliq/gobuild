// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2020-2021, Ctrl IQ, Inc. All rights reserved

package gobuild

import (
	"errors"
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

var onceGitDescribe sync.Once
var cachedGitDescription *GitDescription
var cachedGitDescriptionError error

type GitDescription struct {
	isClean bool                // if true, the git working tree has local modifications
	ref     *plumbing.Reference // reference being described
	tag     *object.Tag         // nearest semver tag reachable from ref (or nil if none found)
	n       uint64              // number of commits between nearest semver tag and ref (if tag is non-nil)
}

func GitDescribe() (*GitDescription, error) {
	onceGitDescribe.Do(func() {
		// Open git repo.
		repo, err := git.PlainOpen(".")
		if err != nil {
			cachedGitDescriptionError = err
			return
		}

		// Get HEAD commit.
		head, err := repo.Head()
		if err != nil {
			cachedGitDescriptionError = err
			return
		}

		cachedGitDescription, err = describe(repo, head)
		cachedGitDescriptionError = err
	})

	return cachedGitDescription, cachedGitDescriptionError
}

// GetSemver returns a semantic version based on d.
func (gd *GitDescription) GetSemver() (semver.Version, error) {
	if gd.tag == nil {
		return semver.Version{}, errors.New("no semver tags found")
	}

	v, err := semver.Parse(gd.tag.Name[1:])
	if err != nil {
		return semver.Version{}, err
	}

	// If this version wasn't tagged directly, modify tag.
	if gd.n > 0 {
		if len(v.Pre) == 0 {
			// The tag is not a pre-release version. Bump the patch version and add a pre-release
			// of alpha.0. Semantically, this indicates this is pre-alpha.1, which would normally
			// be the first alpha version.
			v.Patch++
			v.Pre = append(v.Pre, semver.PRVersion{VersionStr: fmt.Sprintf("alpha.%d", v.Patch)})
		}

		// Append devel.N to pre-release version. For example, if the tag is 0.1.2-alpha.1, tag as
		// 0.1.2-alpha.1.devel.3. Semantically, this indicates this version is between alpha.1 and
		// alpha.2.
		v.Pre = append(v.Pre, semver.PRVersion{VersionStr: fmt.Sprintf("devel.%d", gd.n)})
	}

	return v, nil
}

func (gd *GitDescription) ListEntries() []string {
	if gd.tag == nil {
		return nil
	}
	tree, err := gd.tag.Tree()
	if err != nil {
		return nil
	}

	entries := make([]string, 0)

	tw := object.NewTreeWalker(tree, true, nil)
	defer tw.Close()

	for {
		name, _, err := tw.Next()
		if err != nil {
			break
		}

		entries = append(entries, name)
	}

	return entries
}

// getVersionTags returns a map of commit hashes to tags.
func getVersionTags(r *git.Repository) (map[plumbing.Hash]*object.Tag, error) {
	// Get a list of tags. Note that we cannot use r.TagObjects() directly, since that returns
	// objects that are not referenced (for example, deleted tags.)
	tagIter, err := r.Tags()
	if err != nil {
		return nil, err
	}

	// Iterate through tags, selecting tags that match regex.
	tags := make(map[plumbing.Hash]*object.Tag)
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if len(name) == 0 {
			return nil
		}
		if _, err := semver.Parse(name[1:]); err == nil {
			t, err := r.TagObject(ref.Hash())
			if err != nil {
				return err
			}
			tags[t.Target] = t
		}
		return nil
	})

	return tags, err
}

// describe returns a gitDescription of ref.
func describe(r *git.Repository, ref *plumbing.Reference) (*GitDescription, error) {
	w, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree: %s", err)
	}

	status, err := w.Status()
	if err != nil {
		return nil, fmt.Errorf("worktree status: %s", err)
	}

	// Get version tags.
	tags, err := getVersionTags(r)
	if err != nil {
		return nil, fmt.Errorf("version tag: %s", err)
	}

	// Get commit log.
	logIter, err := r.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		From:  ref.Hash(),
	})
	if err != nil {
		return nil, err
	}

	gd := &GitDescription{
		isClean: status.IsClean(),
		ref:     ref,
	}

	// Iterate through commit log until we find a matching tag.
	err = logIter.ForEach(func(c *object.Commit) error {
		if t, ok := tags[c.Hash]; ok {
			gd.tag = t
			return storer.ErrStop
		}
		gd.n++
		return nil
	})
	if err != nil {
		return nil, err
	}

	return gd, nil
}
