// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"
)

// BuildRootfs generates a rootfs based on the provided working directory and
// the rootfs entrypoint for the provided target(s).
func BuildRootfs(ctx context.Context, workdir, rootfs string, compress bool, targets ...target.Target) (string, [][]string, [][]string, error) {
	if rootfs == "" {
		return "", nil, nil, nil
	}

	var processes []*processtree.ProcessTreeItem
	var archs []string
	var cmds [][]string
	var envs [][]string

	for _, targ := range targets {
		arch := targ.Architecture().String()
		if slices.Contains[[]string](archs, arch) {
			continue
		}

		archs = append(archs, arch)

		if !filepath.IsAbs(rootfs) {
			rootfs = filepath.Join(workdir, rootfs)
		}

		ramfs, err := initrd.New(ctx, rootfs,
			initrd.WithOutput(filepath.Join(
				workdir,
				unikraft.BuildDir,
				fmt.Sprintf(initrd.DefaultInitramfsArchFileName, arch),
			)),
			initrd.WithCacheDir(filepath.Join(
				workdir,
				unikraft.VendorDir,
				"rootfs-cache",
			)),
			initrd.WithArchitecture(arch),
			initrd.WithCompression(compress),
		)
		if err != nil {
			return "", nil, nil, fmt.Errorf("could not initialize initramfs builder: %w", err)
		}

		processes = append(processes,
			processtree.NewProcessTreeItem(
				"building rootfs",
				arch,
				func(ctx context.Context) error {
					// TODO
					rootfs, err = ramfs.Build(ctx)
					if err != nil {
						return err
					}

					cmds = append(cmds, ramfs.Args())
					envs = append(envs, ramfs.Env())

					return nil
				},
			),
		)
	}

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
		},
		processes...,
	)
	if err != nil {
		return "", nil, nil, err
	}

	if err := model.Start(); err != nil {
		return "", nil, nil, err
	}

	return rootfs, cmds, envs, nil
}
