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

	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type removeImageResponse struct {
}

func imageRemoveHandler(c *cli.Context) error {

	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "rmi")
		return errors.New("Exactly one arguments expected")
	}

	imageName := c.Args()[0]
	logrus.Debugf("Remove Image Request: %+v", imageName)

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

	images, err := imageService.ResolveNames(imageName)
	if err != nil {
		if err == ErrCannotParseImageID {
			images = append(images, imageName)
		} else {
			return err
		}
	}

	var deleted bool

	for _, img := range images {
		err = imageService.UntagImage(&types.SystemContext{}, img)
		if err != nil {
			logrus.Debugf("error deleting image %s: %v", img, err)
			continue
		}
		deleted = true
		break
	}
	if !deleted && err != nil {
		return err
	}

	resp := &removeImageResponse{}
	logrus.Debugf("removeImageResponse: %+v", resp)

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", data)
	return err
}

var imageRemoveCmd = cli.Command{
	Name:  "rmi",
	Usage: "isulad_kit rmi [ID|NAME]",
	Description: fmt.Sprintf(`

	Remove one image.

	`),
	ArgsUsage: "[ID|NAME]",
	Action:    imageRemoveHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{},
}
