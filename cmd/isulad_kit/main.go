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
	"context"
	"fmt"
	"os"

	"github.com/containers/image/signature"
	cstorage "github.com/containers/storage"
	"github.com/containers/storage/pkg/parsers"
	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// gitCommit will be the hash
var gitCommit = ""

const (
	defaultTransport       = "docker://"
	defaultRunRoot         = "/var/run/containers/storage"
	defaultGraphRoot       = "/var/lib/containers/storage"
	defaultGraphDriverName = "overlay"
)

// createApp returns a cli.App to be run or tested.
func createApp() *cli.App {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Name = "isulad_kit"
	if gitCommit != "" {
		app.Version = fmt.Sprintf("%s commit: %s", Version, gitCommit)
	} else {
		app.Version = Version
	}
	app.Usage = "Various operations with container images and container image registries"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output",
		},
		cli.StringFlag{
			Name:  "log-level, l",
			Usage: "Set the logging level",
		},
		cli.StringFlag{
			Name:  "run-root",
			Value: defaultRunRoot,
			Usage: "use `PATH` as the root directory for execution state files",
		},
		cli.StringFlag{
			Name:  "graph-root",
			Value: defaultGraphRoot,
			Usage: "use `PATH` as the graph driver's root directory for execution state files",
		},
		cli.StringFlag{
			Name:  "driver-name",
			Value: defaultGraphDriverName,
			Usage: "use `NAME` as the graph driver",
		},
		cli.StringSliceFlag{
			Name:  "driver-options",
			Usage: "Options of the graph driver",
		},
		cli.StringSliceFlag{
			Name:  "storage-opt",
			Usage: "Options of the storage when mount container rootfs",
		},
		cli.StringSliceFlag{
			Name:  "insecure-registry",
			Usage: "whether to disable TLS verification for the given registry",
		},
		cli.StringSliceFlag{
			Name:  "registry",
			Usage: "registry to be prepended when pulling unqualified images, can be specified multiple times",
		},
		cli.StringFlag{
			Name:  "policy",
			Value: "",
			Usage: "Path to a trust policy file",
		},
		cli.BoolFlag{
			Name:  "insecure-policy",
			Usage: "run the tool without any policy check",
		},
		cli.DurationFlag{
			Name:  "command-timeout",
			Usage: "timeout for the command execution",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			setLogLevel(c.GlobalString("log-level"))
		}
		return nil
	}
	app.Commands = []cli.Command{
		pullCmd,
		infoCmd,
		imagesCmd,
		imageStatusCmd,
		imageRemoveCmd,
		imageFsinfoCmd,
		mountCmd,
		uMountCmd,
		containerPrepareCmd,
		containerRemoveCmd,
		copyCmd,
		storageStatusCmd,
		storageUmountCmd,
		containerFilesystemUsageCmd,
		exportCmd,
		loadCmd,
		loginCmd,
		logoutCmd,
	}
	return app
}

func main() {
	if reexec.Init() {
		return
	}
	app := createApp()
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func getStorageStore(readonly bool, c *cli.Context) (cstorage.Store, error) {
	return cstorage.GetStore(cstorage.StoreOptions{
		RunRoot:            c.GlobalString("run-root"),
		GraphRoot:          c.GlobalString("graph-root"),
		GraphDriverName:    c.GlobalString("driver-name"),
		GraphDriverOptions: c.GlobalStringSlice("driver-options"),
		ReadOnly:           readonly,
	})
}

func getEmptyStorageStore(c *cli.Context) (cstorage.Store, error) {
	return cstorage.GetStore(cstorage.StoreOptions{
		RunRoot:            c.GlobalString("run-root"),
		GraphRoot:          c.GlobalString("graph-root"),
		GraphDriverName:    c.GlobalString("driver-name"),
		GraphDriverOptions: c.GlobalStringSlice("driver-options"),
		ReadOnly:           true,
		DonotLoadData:      true,
	})
}

func getImageService(ctx context.Context, c *cli.Context, store cstorage.Store) (ImageServer, error) {
	return InitImageService(ctx, store, defaultTransport,
		c.GlobalStringSlice("insecure-registry"), c.GlobalStringSlice("registry"))
}

func getRuntimeService(ctx context.Context, pauseImage string, imageService ImageServer) ContainerServer {
	return GetContainerLifeService(ctx, imageService, pauseImage)
}

// getPolicyContext handles the global "policy" flag.
func getPolicyContext(c *cli.Context) (*signature.PolicyContext, error) {
	policyPath := c.GlobalString("policy")
	var policy *signature.Policy // This could be cached across calls, if we had an application context.
	var err error
	if c.GlobalBool("insecure-policy") {
		policy = &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	} else if policyPath == "" {
		policy, err = signature.DefaultPolicy(nil)
	} else {
		policy, err = signature.NewPolicyFromFile(policyPath)
	}
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
}

func setLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse logging level: %s\n", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func getStorageOptions(c *cli.Context) (map[string]string, error) {
	storageOpts := make(map[string]string)
	options := c.GlobalStringSlice("storage-opt")
	for _, opt := range options {
		key, val, err := parsers.ParseKeyValueOpt(opt)
		if err != nil {
			return nil, err
		}
		storageOpts[key] = val
	}
	return storageOpts, nil
}
