// Copyright (c) Huawei Technologies Co., Ltd. 2019-2020. All rights reserved.
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

// Since some of this code is derived from cri-o, their copyright
// is retained here....
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// The original version of this file can be found at
// https://github.com/cri-o/cri-o/blob/master/internal/pkg/storage/runtime.go

package main

import (
	"context"
	"encoding/json"
	"time"

	imstorage "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	constorage "github.com/containers/storage"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrImageName invalid image name
	ErrImageName = errors.New("invalid image name")
	// ErrContainerName invalid container name
	ErrContainerName = errors.New("invalid container name")
	// ErrContainerID invalid container ID
	ErrContainerID = errors.New("invalid container ID")
)

type containerLifeService struct {
	imageServer ImageServer
	pauseImage  string
	ctx         context.Context
}

// ContainerSpec ContainerSpec
type ContainerSpec struct {
	Config     *v1.Image
	ID         string
	RootDir    string
	RunDir     string
	MountPoint string
	LayerID    string
}

// ContainerServer display all related operations
type ContainerServer interface {
	// GetContainer
	GetContainerLayerID(id string) (string, error)
	// CreateContainer
	CreateContainer(systemContext *types.SystemContext, imageName, imageID, containerName, containerID string, storageOpts map[string]string) (ContainerSpec, error)
	// RemoveContainer
	RemoveContainer(containerID string) error
	// UmountContainer
	UmountContainer(containerID string, force bool) error
}

func (r *containerLifeService) createContainer(systemContext *types.SystemContext, imageName, imageID, containerName, containerID string, storageOpts map[string]string) (ContainerSpec, error) {
	var ref types.ImageReference

	if imageName == "" && imageID == "" {
		return ContainerSpec{}, ErrImageName
	}
	if containerName == "" {
		return ContainerSpec{}, ErrContainerName
	}

	metadataName := containerName

	ref, err := imstorage.Transport.ParseStoreReference(r.imageServer.GetStore(), imageName)
	if err != nil {
		otherRef, err2 := alltransports.ParseImageName(imageName)
		if err2 == nil && otherRef.DockerReference() != nil {
			ref, err = imstorage.Transport.ParseStoreReference(r.imageServer.GetStore(), otherRef.DockerReference().Name())
		}
		if err != nil {
			ref, err = imstorage.Transport.ParseStoreReference(r.imageServer.GetStore(), "@"+imageID)
			if err != nil {
				return ContainerSpec{}, err
			}
		}
	}

	img, err := imstorage.Transport.GetStoreImage(r.imageServer.GetStore(), ref)
	if err != nil {
		return ContainerSpec{}, err
	}

	image, err := ref.NewImage(r.ctx, systemContext)
	if err != nil {
		return ContainerSpec{}, err
	}
	defer image.Close()

	imageConfig, err := image.OCIConfig(r.ctx)
	if err != nil {
		return ContainerSpec{}, err
	}

	if imageName == "" && len(img.Names) > 0 {
		imageName = img.Names[0]
	}
	imageID = img.ID

	metadata := constorage.RuntimeContainerMetadata{
		Pod:           false,
		PodName:       containerName,
		PodID:         containerName,
		ImageName:     imageName,
		ImageID:       imageID,
		ContainerName: containerName,
		MetadataName:  metadataName,
		UID:           "",
		Namespace:     "",
		Attempt:       0,
		CreatedAt:     time.Now().Unix(),
		MountLabel:    "",
	}
	mdata, err := json.Marshal(&metadata)
	if err != nil {
		return ContainerSpec{}, err
	}

	names := []string{metadata.ContainerName}
	if metadata.Pod {
		names = append(names, metadata.PodName)
	}

	coptions := &constorage.ContainerOptions{}

	coptions.Flags = make(map[string]interface{})
	if storageOpts != nil {
		tmpStorageOpts := make(map[string]string)
		for k, v := range storageOpts {
			tmpStorageOpts[k] = v
		}
		coptions.Flags["StorageOpts"] = tmpStorageOpts
	}

	container, err := r.imageServer.GetStore().CreateContainer(containerID, names, img.ID, "", string(mdata), coptions)
	if err != nil {
		return ContainerSpec{}, err
	}

	defer func() {
		if err != nil {
			if err2 := r.imageServer.GetStore().DeleteContainer(container.ID); err2 != nil {
				return
			}
		}
	}()

	layerName := metadata.ContainerName + "-layer"
	names, err = r.imageServer.GetStore().Names(container.LayerID)
	if err != nil {
		return ContainerSpec{}, err
	}
	names = append(names, layerName)
	err = r.imageServer.GetStore().SetNames(container.LayerID, names)
	if err != nil {
		return ContainerSpec{}, err
	}

	containerRootDir, err := r.imageServer.GetStore().ContainerDirectory(container.ID)
	if err != nil {
		return ContainerSpec{}, err
	}

	containerRunDir, err := r.imageServer.GetStore().ContainerRunDirectory(container.ID)
	if err != nil {
		return ContainerSpec{}, err
	}

	return ContainerSpec{
		ID:         container.ID,
		RootDir:    containerRootDir,
		RunDir:     containerRunDir,
		Config:     imageConfig,
		MountPoint: container.MountPoint,
		LayerID:    container.LayerID,
	}, nil
}

func (r *containerLifeService) CreateContainer(systemContext *types.SystemContext, imageName, imageID, containerName, containerID string, storageOpts map[string]string) (ContainerSpec, error) {
	return r.createContainer(systemContext, imageName, imageID, containerName, containerID, storageOpts)
}

func (r *containerLifeService) RemoveContainer(containerID string) error {
	if containerID == "" {
		return ErrContainerID
	}
	container, err := r.imageServer.GetStore().Container(containerID)
	if err != nil {
		return err
	}
	err = r.imageServer.GetStore().DeleteContainer(container.ID)
	if err != nil {
		logrus.Debugf("failed to delete container %q: %v", container.ID, err)
		return err
	}
	logrus.Debugf("container %q deleted", container.ID)
	return nil
}

func (r *containerLifeService) GetContainerLayerID(id string) (string, error) {
	container, err := r.imageServer.GetStore().Container(id)
	if err != nil {
		if errors.Cause(err) == storage.ErrContainerUnknown {
			return "", ErrContainerID
		}
		return "", err
	}
	logrus.Debugf("container %q layer id is: %q", container.ID, container.LayerID)
	return container.LayerID, nil
}

func (r *containerLifeService) UmountContainer(containerID string, force bool) error {
	if containerID == "" {
		return ErrContainerID
	}
	container, err := r.imageServer.GetStore().Container(containerID)
	if err != nil {
		return err
	}
	_, err = r.imageServer.GetStore().Unmount(container.ID, force)
	if err != nil {
		logrus.Debugf("failed to unmount container %q: %v", container.ID, err)
		return err
	}
	logrus.Debugf("container %q unmounted", container.ID)
	return nil
}

// GetContainerLifeService returns a ContainerServer
func GetContainerLifeService(ctx context.Context, imageServer ImageServer, pauseImage string) ContainerServer {
	return &containerLifeService{
		imageServer: imageServer,
		pauseImage:  pauseImage,
		ctx:         ctx,
	}
}
