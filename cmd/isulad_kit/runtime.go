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
	"encoding/json"
	"time"

	"github.com/containers/image/copy"
	istorage "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	cstorage "github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrInvalidPodName invalid pod name
	ErrInvalidPodName = errors.New("invalid pod name")
	// ErrInvalidImageName invalid image name
	ErrInvalidImageName = errors.New("invalid image name")
	// ErrInvalidContainerName invalid container name
	ErrInvalidContainerName = errors.New("invalid container name")
	// ErrInvalidSandboxID invalid sandbox ID
	ErrInvalidSandboxID = errors.New("invalid sandbox ID")
	// ErrInvalidContainerID invalid container ID
	ErrInvalidContainerID = errors.New("invalid container ID")
)

type runtimeService struct {
	storageImageServer ImageServer
	pauseImage         string
	ctx                context.Context
}

// ContainerInfo ContainerInfo
type ContainerInfo struct {
	ID     string
	Dir    string
	RunDir string
	Config *v1.Image
}

// RuntimeServer wraps up various CRI-related activities into a reusable
// implementation.
type RuntimeServer interface {
	// GetContainer
	GetContainerLayerID(id string) (string, error)
	// CreateContainer
	CreateContainer(systemContext *types.SystemContext, podName, podID, imageName, imageID, containerName, containerID, metadataName string, attempt uint32, mountLabel string, idMappings *idtools.IDMappings, storageOpts map[string]string, copyOptions *copy.Options) (ContainerInfo, error)
	// RemoveContainer
	RemoveContainer(idOrName string) error
	// UmountContainer
	UmountContainer(idOrName string) error
}

func (r *runtimeService) createContainer(systemContext *types.SystemContext, podName, podID, imageName, imageID, containerName, containerID, metadataName, uid, namespace string, attempt uint32, mountLabel string, idMappings *idtools.IDMappings, storageOpts map[string]string, options *copy.Options) (ContainerInfo, error) {
	var ref types.ImageReference
	if podName == "" || podID == "" {
		return ContainerInfo{}, ErrInvalidPodName
	}
	if imageName == "" && imageID == "" {
		return ContainerInfo{}, ErrInvalidImageName
	}
	if containerName == "" {
		return ContainerInfo{}, ErrInvalidContainerName
	}
	if metadataName == "" {
		metadataName = containerName
	}

	ref, err := istorage.Transport.ParseStoreReference(r.storageImageServer.GetStore(), imageName)
	if err != nil {
		otherRef, err2 := alltransports.ParseImageName(imageName)
		if err2 == nil && otherRef.DockerReference() != nil {
			ref, err = istorage.Transport.ParseStoreReference(r.storageImageServer.GetStore(), otherRef.DockerReference().Name())
		}
		if err != nil {
			ref, err = istorage.Transport.ParseStoreReference(r.storageImageServer.GetStore(), "@"+imageID)
			if err != nil {
				return ContainerInfo{}, err
			}
		}
	}

	img, err := istorage.Transport.GetStoreImage(r.storageImageServer.GetStore(), ref)
	if err != nil {
		return ContainerInfo{}, err
	}

	image, err := ref.NewImage(r.ctx, systemContext)
	if err != nil {
		return ContainerInfo{}, err
	}
	defer image.Close()

	imageConfig, err := image.OCIConfig(r.ctx)
	if err != nil {
		return ContainerInfo{}, err
	}

	if imageName == "" && len(img.Names) > 0 {
		imageName = img.Names[0]
	}
	imageID = img.ID

	metadata := cstorage.RuntimeContainerMetadata{
		Pod:           containerID == podID,
		PodName:       podName,
		PodID:         podID,
		ImageName:     imageName,
		ImageID:       imageID,
		ContainerName: containerName,
		MetadataName:  metadataName,
		UID:           uid,
		Namespace:     namespace,
		Attempt:       attempt,
		CreatedAt:     time.Now().Unix(),
		MountLabel:    mountLabel,
	}
	mdata, err := json.Marshal(&metadata)
	if err != nil {
		return ContainerInfo{}, err
	}

	names := []string{metadata.ContainerName}
	if metadata.Pod {
		names = append(names, metadata.PodName)
	}

	coptions := &cstorage.ContainerOptions{}
	if idMappings != nil {
		coptions.IDMappingOptions = cstorage.IDMappingOptions{UIDMap: idMappings.UIDs(), GIDMap: idMappings.GIDs()}
	}

	coptions.Flags = make(map[string]interface{})
	if storageOpts != nil {
		coptions.Flags["StorageOpts"] = storageOpts
	}

	container, err := r.storageImageServer.GetStore().CreateContainer(containerID, names, img.ID, "", string(mdata), coptions)
	if err != nil {
		return ContainerInfo{}, err
	}

	defer func() {
		if err != nil {
			if err2 := r.storageImageServer.GetStore().DeleteContainer(container.ID); err2 != nil {
				return
			}
		}
	}()

	layerName := metadata.ContainerName + "-layer"
	names, err = r.storageImageServer.GetStore().Names(container.LayerID)
	if err != nil {
		return ContainerInfo{}, err
	}
	names = append(names, layerName)
	err = r.storageImageServer.GetStore().SetNames(container.LayerID, names)
	if err != nil {
		return ContainerInfo{}, err
	}

	containerDir, err := r.storageImageServer.GetStore().ContainerDirectory(container.ID)
	if err != nil {
		return ContainerInfo{}, err
	}

	containerRunDir, err := r.storageImageServer.GetStore().ContainerRunDirectory(container.ID)
	if err != nil {
		return ContainerInfo{}, err
	}

	return ContainerInfo{
		ID:     container.ID,
		Dir:    containerDir,
		RunDir: containerRunDir,
		Config: imageConfig,
	}, nil
}

func (r *runtimeService) CreateContainer(systemContext *types.SystemContext, podName, podID, imageName, imageID, containerName, containerID, metadataName string, attempt uint32, mountLabel string, idMappings *idtools.IDMappings, storageOpts map[string]string, copyOptions *copy.Options) (ContainerInfo, error) {
	return r.createContainer(systemContext, podName, podID, imageName, imageID, containerName, containerID, metadataName, "", "", attempt, mountLabel, idMappings, storageOpts, copyOptions)
}

func (r *runtimeService) RemoveContainer(idOrName string) error {
	if idOrName == "" {
		return ErrInvalidContainerID
	}
	container, err := r.storageImageServer.GetStore().Container(idOrName)
	if err != nil {
		return err
	}
	err = r.storageImageServer.GetStore().DeleteContainer(container.ID)
	if err != nil {
		logrus.Debugf("failed to delete container %q: %v", container.ID, err)
		return err
	}
	return nil
}

func (r *runtimeService) GetContainerLayerID(id string) (string, error) {
	container, err := r.storageImageServer.GetStore().Container(id)
	if err != nil {
		if errors.Cause(err) == storage.ErrContainerUnknown {
			return "", ErrInvalidContainerID
		}
		return "", err
	}
	logrus.Debugf("container %q layer id is: %q", container.ID, container.LayerID)
	return container.LayerID, nil
}

func (r *runtimeService) UmountContainer(idOrName string) error {
	if idOrName == "" {
		return ErrInvalidContainerID
	}
	container, err := r.storageImageServer.GetStore().Container(idOrName)
	if err != nil {
		return err
	}
	_, err = r.storageImageServer.GetStore().Unmount(container.ID, false)
	if err != nil {
		logrus.Debugf("failed to unmount container %q: %v", container.ID, err)
		return err
	}
	logrus.Debugf("unmounted container %q", container.ID)
	return nil
}

// GetRuntimeService returns a RuntimeServer
func GetRuntimeService(ctx context.Context, storageImageServer ImageServer, pauseImage string) RuntimeServer {
	return &runtimeService{
		storageImageServer: storageImageServer,
		pauseImage:         pauseImage,
		ctx:                ctx,
	}
}
