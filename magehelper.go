// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2020-2021, Ctrl IQ, Inc. All rights reserved

package gobuild

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func goCmd(args []string) error {
	return sh.RunV(mg.GoCmd(), args...)
}

func RunIntegration(paths ...string) error {
	args := []string{"test", "-count", "1", "-cover", "-race"}
	args = append(args, paths...)
	return goCmd(args)
}

func RunUnitTest(paths ...string) error {
	args := []string{"test", "-short", "-count", "1", "-cover", "-race"}
	args = append(args, paths...)
	return goCmd(args)
}

func RunInstall(args ...string) error {
	a := []string{"install"}
	a = append(a, args...)
	return goCmd(a)
}

func RunBuild(args ...string) error {
	a := []string{"build"}
	a = append(a, args...)
	return goCmd(a)
}
