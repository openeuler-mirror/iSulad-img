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
	"encoding/json"
	"fmt"
	"time"

	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Int64Value is the wrapper of int64.
type Int64Value struct {
	// The value.
	Value int64 `json:"value,omitempty"`
}

// Image provide Basic information about a container image.
type Image struct {
	// ID of the image.
	ID string `json:"id,omitempty"`
	// Other names by which this image is known.
	RepoTags []string `json:"repo_tags,omitempty"`
	// Digests by which this image is known.
	RepoDigests []string `json:"repo_digests,omitempty"`
	// Size of the image in bytes. Must be > 0.
	Size uint64 `json:"size,omitempty"`
	// UID that will run the command(s). This is used as a default if no user is
	// specified when creating the container. UID and the following user name
	// are mutually exclusive.
	UID *Int64Value `json:"uid,omitempty"`
	// User name that will run the command(s). This is used if UID is not set
	// and no user is specified when creating container.
	Username string `json:"username,omitempty"`
	// Created is the combined date and time at which the image was created, formatted as defined by RFC 3339, section 5.6.
	Created *time.Time `json:"created,omitempty"`
	// Loaded is the combined date and time at which the image was pulled, formatted as defined by RFC 3339, section 5.6.
	Loaded *time.Time `json:"Loaded,omitempty"`

	ImageSpec *v1.Image `json:"Spec,omitempty"`

	Healthcheck *HealthConfig
}

type imageStatusResponse struct {
	// Status of the image.
	Image *Image `json:"image,omitempty"`
	// Info is extra information of the Image. The key could be arbitrary string, and
	// value should be in json format. The information could include anything useful
	// for debug, e.g. image config for oci image based container runtime.
	// It should only be returned non-empty when Verbose is true.
	Info map[string]string
}

func imageStatusHandler(c *cli.Context) error {

	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "status")
		return errors.New("Exactly one arguments expected")
	}

	imageName := c.Args()[0]
	logrus.Debugf("Status Image Request: %+v", imageName)

	store, err := getStorageStore(true, c)
	if err != nil {
		return err
	}

	ctx, cancel := commandTimeoutContextFromGlobalOptions(c)
	defer cancel()

	imageService, err := getImageService(ctx, c, store)
	if err != nil {
		return err
	}

	images, err := imageService.ParseImageNames(imageName)
	if err != nil {
		if err == ErrParseImageID {
			images = append(images, imageName)
		} else {
			return err
		}
	}

	var (
		notfound bool
		lastErr  error
	)
	resp := &imageStatusResponse{}
	for _, image := range images {
		status, err := imageService.GetOneImage(&types.SystemContext{}, image)
		if err != nil {
			if errors.Cause(err) == storage.ErrImageUnknown {
				logrus.Warnf("imageStatus: can't find %s", image)
				notfound = true
				continue
			}
			logrus.Warnf("imageStatus: error getting status from %s: %v", image, err)
			lastErr = err
			continue
		}
		created := *status.Created
		loaded := *status.Loaded
		imageConfig, err := getImageConf(store, image)
		if err != nil {
			return err
		}
		healthcheck, err := getHealthcheck(store, image)
		if err != nil {
			return err
		}
		resp = &imageStatusResponse{
			Image: &Image{
				ID:          status.ID,
				RepoTags:    status.RepoTags,
				RepoDigests: status.RepoDigests,
				Size:        *status.Size,
				Created:     &created,
				Loaded:      &loaded,
				ImageSpec:   imageConfig,
				Healthcheck: healthcheck,
			},
		}
		uid, username := getUserFromImage(status.User)
		if uid != nil {
			resp.Image.UID = &Int64Value{Value: *uid}
		}
		resp.Image.Username = username
		break
	}
	if lastErr != nil && resp == nil {
		return lastErr
	}
	if notfound && resp == nil {
		resp = &imageStatusResponse{}
	}

	logrus.Debugf("StatusImagesResponse: %+v", resp)

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", data)
	return err
}

var imageStatusCmd = cli.Command{
	Name:  "status",
	Usage: "isulad_kit status [ID|NAME]",
	Description: fmt.Sprintf(`

	Display detailed information of the image.

	`),
	ArgsUsage: "[ID|NAME]",
	Action:    imageStatusHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{},
}
