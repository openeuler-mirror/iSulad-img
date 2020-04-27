// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Description: iSulad image kit
// Author: lifeng
// Create: 2019-05-06

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/urfave/cli"
)

type loadOptions struct {
	input string
	tag   string
}

func getLoadOptions(c *cli.Context) *loadOptions {
	return &loadOptions{
		input: c.String("input"),
		tag:   c.String("tag"),
	}
}

func allRepoTags(images []tarfile.ManifestItem) []string {
	var repoTags []string
	for _, image := range images {
		if image.RepoTags != nil {
			repoTags = append(repoTags, image.RepoTags...)
		}
	}
	return repoTags
}

func getTagFromArchive(tar *tarfile.Source) ([]string, error) {
	manifest, err := tar.LoadTarManifest()
	if err != nil {
		return nil, err
	}

	allTags := allRepoTags(manifest)
	if len(allTags) == 0 {
		return nil, errors.New("No tag found in archive")
	}

	return allTags, nil
}

func loadImage(gopts *globalOptions, lopts *loadOptions) (string, error) {
	policyContext, err := getPolicyContext(gopts)
	if err != nil {
		return "", fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	input := lopts.input
	if input == "" {
		return "", fmt.Errorf("Missing input parameter, use --input to specify input file")
	}

	// If tar is compressed, NewSourceFromFile will decompress it and we should use the
	// temporary decompressed tar file as input to avoid re-decompress later.
	tar, err := tarfile.NewSourceFromFile(input, "")
	if err != nil {
		return "", err
	}
	defer tar.Close()

	tags, err := getTagFromArchive(tar)
	if err != nil {
		return "", fmt.Errorf("Get tag from input file %s failed: %v", input, err)
	}
	tag := lopts.tag
	if len(tags) > 1 && tag != "" {
		return "", fmt.Errorf("Can not use --tag option because more than one image found in tar archive")
	}

	store, err := getStorageStore(gopts)
	if err != nil {
		return "", err
	}

	var output string
	for _, srcTag := range tags {
		destTag := srcTag
		if tag != "" {
			destTag = tag
		}

		// Format: docker-archive:/path/image.tar:imagename:v1
		formatedInput := "docker-archive:" + tarfile.TarPath(tar) + ":" + srcTag
		srcRef, err := alltransports.ParseImageName(formatedInput)
		if err != nil {
			return output, fmt.Errorf("Invalid input name %s: %v", formatedInput, err)
		}

		destRef, err := storage.Transport.ParseStoreReference(store, destTag)
		if err != nil {
			return output, fmt.Errorf("Invalid tag %s: %v", destTag, err)
		}

		_, err = copy.Image(context.Background(), policyContext, destRef, srcRef, &copy.Options{
			ReportWriter: os.Stdout,
		})
		if err != nil {
			return output, fmt.Errorf("Load image %v failed: %v", srcTag, err)
		}

		loadedOneImage := fmt.Sprintf("Loaded image: %s\n", destRef.DockerReference().String())
		fmt.Fprintf(os.Stdout, loadedOneImage)
		output += loadedOneImage
	}

	return output, err
}
