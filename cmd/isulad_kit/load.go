// Copyright (c) Huawei Technologies Co., Ltd. 2019-2019. All rights reserved.
// iSulad-kit licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad image kit
// Author: lifeng
// Create: 2019-05-06

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/urfave/cli"
)

func getTagFromArchive(input string) (string, error) {
	/* Use package tarfile to avoid unpack tar file and avoid use /tmp,
	so we can reduce disk usage and memory usage during load. */
	tar, err := tarfile.NewSourceFromFile(input)
	if err != nil {
		return "", err
	}
	defer tar.Close()

	manifest, err := tar.LoadTarManifest()
	if err != nil {
		return "", err
	}

	if len(manifest) == 0 || len(manifest[0].RepoTags) == 0 {
		return "", errors.New("No tag found in archive")
	}

	/* Support load one image with one tag, other tags are ignored. */
	return manifest[0].RepoTags[0], nil
}

func loadHandler(c *cli.Context) error {
	var src string

	if len(c.Args()) != 0 {
		cli.ShowCommandHelp(c, "load")
		return errors.New("Exactly zero arguments expected")
	}

	policyContext, err := getPolicyContext(c)
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	input := c.String("input")
	if input != "" {
		src = "docker-archive:" + input
	} else {
		return fmt.Errorf("Missing input parameter, use --input to specify input file")
	}

	srcRef, err := alltransports.ParseImageName(src)
	if err != nil {
		return fmt.Errorf("Invalid input name %s: %v", input, err)
	}

	tag := c.String("tag")
	if tag == "" {
		tag, err = getTagFromArchive(input)
		if err != nil {
			return fmt.Errorf("Get tag from input file %s failed: %v", input, err)
		}
	}

	store, err := getStorageStore(true, c)
	if err != nil {
		return err
	}

	destRef, err := storage.Transport.ParseStoreReference(store, tag)
	if err != nil {
		return fmt.Errorf("Invalid tag %s: %v", tag, err)
	}

	ctx, cancel := commandTimeoutContextFromGlobalOptions(c)
	defer cancel()

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
		ReportWriter: os.Stdout,
	})

	fmt.Fprintf(os.Stdout, "Loaded image: %s\n", destRef.DockerReference().String())

	return err
}

var loadCmd = cli.Command{
	Name:  "load",
	Usage: "Load an image from a tar",
	Description: fmt.Sprintf(`

	Load an image from a tar
	`),
	ArgsUsage: "",
	Action:    loadHandler,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "input",
			Value: "",
			Usage: "Read from a tar archive file",
		},
		cli.StringFlag{
			Name:  "tag",
			Value: "",
			Usage: "Name and optionally a tag in the 'name:tag' format",
		},
	},
}
