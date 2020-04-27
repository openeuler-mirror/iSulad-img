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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func infoHandler(c *cli.Context) error {
	if len(c.Args()) != 1 {
		cli.ShowCommandHelp(c, "info")
		return errors.New("Exactly one arguments expected")
	}

	image := c.Args()[0]
	logrus.Debugf("Info Image Request: %+v", image)

	gopts, err := getGlobalOptions(c)
	if err != nil {
		return err
	}

	var data string
	sockAddr, err := isDaemonInstanceExist(defaultInfoFile)
	if strings.Contains(err.Error(), daemonInstanceExist) {
		data, err = grpcCliInfo(sockAddr, image)
	} else if os.IsNotExist(err) {
		data, err = imageInfo(gopts, image)
	}
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", data)
	return err
}

func imageInfo(gopts *globalOptions, image string) (string, error) {
	store, err := getStorageStore(gopts)
	if err != nil {
		return "", err
	}

	imageConfig, err := getImageConf(store, image)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(imageConfig)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var infoCmd = cli.Command{
	Name:  "info",
	Usage: "iSulad-img info [OPTIONS] NAME[:TAG|@DIGEST]",
	Description: fmt.Sprintf(`

	Info the image configuration as per OCI v1 image-spec.
	`),
	ArgsUsage: "NAME[:TAG|@DIGEST]",
	Action:    infoHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{},
}
