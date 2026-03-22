// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bassosimone/vflag"
	"github.com/pkg/browser"
)

func browserMain(_ context.Context, args []string) error {
	// Parse flags.
	fset := vflag.NewFlagSet("2026-03-23-lab browser", vflag.ExitOnError)
	datadir := "data"
	fset.StringVar(&datadir, 'd', "datadir", "The `DIR` containing url.txt.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	if err := fset.Parse(args); err != nil {
		return err
	}

	// Read the base URL saved by the serve command.
	urlFile := filepath.Join(datadir, "url.txt")
	data, err := os.ReadFile(urlFile)
	if err != nil {
		return fmt.Errorf("cannot read %s (is the server running?): %w", urlFile, err)
	}
	baseURL := strings.TrimSpace(string(data))

	return browser.OpenURL(baseURL + "lab/")
}
