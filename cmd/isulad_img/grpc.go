// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-img licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad image kit
// Author: wangfengtu
// Create: 2019-07-12

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "isula-image/isula"

	"github.com/containers/image/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	unixPrefix     = "unix://"
	signalChanSize = 2048
)

type daemonOptions struct {
	gopts   *globalOptions
	Address string
}

type grpcImageService struct {
	daemonOptions
}

func grpcCliInfo(sockAddr string, image string) (string, error) {
	conn, err := grpc.Dial(sockAddr, grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	c := pb.NewImageServiceClient(conn)
	resp, err := c.ImageInfo(context.Background(), &pb.ImageInfoRequest{
		Image: &pb.ImageSpec{Image: image},
	})
	if err != nil {
		return "", err
	}

	return resp.Spec, nil
}

func grpcCliImages(sockAddr string, filter string, check bool) (*listImagesResponse, error) {
	conn, err := grpc.Dial(sockAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	c := pb.NewImageServiceClient(conn)
	pbImages, err := c.ListImages(context.Background(), &pb.ListImagesRequest{
		Filter: &pb.ImageFilter{
			Image: &pb.ImageSpec{Image: filter},
		},
		Check: check,
	})
	if err != nil {
		return nil, err
	}

	respImages := &listImagesResponse{}
	for _, pbImage := range pbImages.Images {
		image, err := transPBImageToImage(pbImage)
		if err != nil {
			return nil, err
		}
		respImages.Images = append(respImages.Images, image)
	}

	return respImages, nil
}

func startGrpcService(opts daemonOptions) error {
	var l net.Listener
	var path string

	if strings.HasPrefix(opts.Address, unixPrefix) {
		path = strings.TrimPrefix(opts.Address, unixPrefix)
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		l, err = net.Listen("unix", path)
		if err != nil {
			logrus.Errorf("Listen at unix address %s failed: %v", path, err)
			return err
		}

		if err := os.Chmod(path, 0600); err != nil {
			logrus.Errorf("Chmod for %s failed: %v", path, err)
			return err
		}
	} else {
		return fmt.Errorf("Listen address %s not supported", opts.Address)
	}

	server := grpc.NewServer()
	pb.RegisterImageServiceServer(server, &grpcImageService{
		daemonOptions: opts,
	})

	// Handle signals
	c := make(chan os.Signal, signalChanSize)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGPIPE)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGTERM, syscall.SIGINT:
				logrus.Infof("Received signal %v", s)
				server.Stop()
				delInfoFile(defaultInfoFile)
			case syscall.SIGPIPE:
				// Ignore pipe broken signal
			}
		}
	}()

	logrus.Infof("iSulad_kit GRPC listen on %s", path)

	return server.Serve(l)
}

func transPBImageToImage(pbImage *pb.Image) (*Image, error) {
	var err error
	var loaded, created time.Time

	if pbImage == nil {
		return nil, nil
	}

	if pbImage.Loaded != "" {
		loaded, err = time.Parse(time.RFC3339Nano, pbImage.Loaded)
		if err != nil {
			return nil, err
		}
	}

	if pbImage.Created != "" {
		created, err = time.Parse(time.RFC3339Nano, pbImage.Created)
		if err != nil {
			return nil, err
		}
	}

	uid := &Int64Value{}
	if pbImage.Uid != nil {
		uid.Value = pbImage.Uid.Value
	} else {
		uid = nil
	}

	respImg := &Image{
		ID:          pbImage.Id,
		RepoTags:    pbImage.RepoTags,
		RepoDigests: pbImage.RepoDigests,
		Size:        pbImage.Size,
		UID:         uid,
		Username:    pbImage.Username,
		Created:     &created,
		Loaded:      &loaded,
	}

	if pbImage.Spec != nil && pbImage.Spec.Image != "" {
		err = json.Unmarshal([]byte(pbImage.Spec.Image), &respImg.ImageSpec)
		if err != nil {
			return nil, err
		}
	}

	return respImg, nil
}

func transImageToPBImage(img *Image) (*pb.Image, error) {
	var err error
	var loaded, created string

	if img == nil {
		return nil, nil
	}

	if img.Loaded != nil {
		loaded = img.Loaded.Format(time.RFC3339Nano)
	}

	if img.Created != nil {
		created = img.Created.Format(time.RFC3339Nano)
	}

	uid := &pb.Int64Value{}
	if img.UID != nil {
		uid.Value = img.UID.Value
	} else {
		uid = nil
	}

	respImg := &pb.Image{
		Id:          img.ID,
		RepoTags:    img.RepoTags,
		RepoDigests: img.RepoDigests,
		Size:        img.Size,
		Uid:         uid,
		Username:    img.Username,
		Created:     created,
		Loaded:      loaded,
	}

	spec, err := json.Marshal(img.ImageSpec)
	if err != nil {
		return nil, err
	}

	if spec != nil {
		respImg.Spec = &pb.ImageSpec{Image: string(spec)}
	}

	return respImg, nil
}

// ListImages lists existing images.
func (s *grpcImageService) ListImages(ctx context.Context, req *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
	var filter string

	if req == nil || req.Filter == nil || req.Filter.Image == nil {
		filter = ""
	} else {
		filter = req.Filter.Image.Image
	}

	images, err := listImages(s.gopts, filter, req.Check)
	if err != nil {
		return &pb.ListImagesResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	resp := &pb.ListImagesResponse{}
	for _, img := range images.Images {
		respImg, err2 := transImageToPBImage(img)
		if err2 != nil {
			return &pb.ListImagesResponse{
				Errmsg: err2.Error(),
				Cc:     1,
			}, err2
		}

		resp.Images = append(resp.Images, respImg)
	}

	return resp, err
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to
// nil.
func (s *grpcImageService) ImageStatus(ctx context.Context, req *pb.ImageStatusRequest) (*pb.ImageStatusResponse, error) {
	if req == nil || req.Image == nil {
		err := errors.New("Lack infomation for image status")
		return &pb.ImageStatusResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	status, err := imageStatus(s.gopts, req.Image.Image)
	if err != nil {
		return &pb.ImageStatusResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	respImg, err := transImageToPBImage(status.Image)
	if err != nil {
		return &pb.ImageStatusResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ImageStatusResponse{Image: respImg}, err
}

// Get image information
func (s *grpcImageService) ImageInfo(ctx context.Context, req *pb.ImageInfoRequest) (*pb.ImageInfoResponse, error) {
	if req == nil || req.Image == nil || req.Image.Image == "" {
		err := errors.New("Lack infomation for image info")
		return &pb.ImageInfoResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	config, err := imageInfo(s.gopts, req.Image.Image)
	if err != nil {
		return &pb.ImageInfoResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ImageInfoResponse{Spec: config}, err
}

// PullImage pulls an image with authentication config.
func (s *grpcImageService) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	if req == nil || req.Image == nil {
		err := errors.New("Lack infomation for pull image")
		return &pb.PullImageResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	popts := &pullOptions{}

	if req.Auth != nil {
		if req.Auth.Username != "" && req.Auth.Password != "" {
			popts.username = req.Auth.Username
			popts.password = req.Auth.Password
		}

		if req.Auth.Auth != "" {
			var err error
			popts.username, popts.password, err = decodeAuth(req.Auth.Auth)
			if err != nil {
				err2 := fmt.Errorf("error decoding authentication for image %s: %v", req.Image.Image, err)
				return &pb.PullImageResponse{
					Errmsg: err2.Error(),
					Cc:     1,
				}, err2
			}
		}
	}

	popts.tlsVerify = s.gopts.TLSVerify

	imageRef, err := imagePull(s.gopts, popts, req.Image.Image)

	return &pb.PullImageResponse{ImageRef: imageRef}, err
}

// RemoveImage removes the image.
// This call is idempotent, and must not return an error if the image has
// already been removed.
func (s *grpcImageService) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	if req == nil || req.Image == nil {
		err := errors.New("Lack infomation for remove image")
		return &pb.RemoveImageResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	err := imageRemove(s.gopts, req.Image.Image)
	if err != nil {
		return &pb.RemoveImageResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.RemoveImageResponse{}, err
}

// Load image from file
func (s *grpcImageService) LoadImage(ctx context.Context, req *pb.LoadImageRequest) (*pb.LoadImageResponose, error) {
	outmsg, err := loadImage(s.gopts, &loadOptions{
		input: req.File,
		tag:   req.Tag,
	})
	if err != nil {
		return &pb.LoadImageResponose{
			Outmsg: outmsg,
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.LoadImageResponose{Outmsg: outmsg}, err
}

func copyImageFsUsage(fsUsage []*FilesystemUsage) (pbFsUsage []*pb.FilesystemUsage) {
	for _, usage := range fsUsage {
		element := &pb.FilesystemUsage{
			Timestamp: usage.Timestamp,
		}

		if usage.FsID != nil {
			element.StorageId = &pb.StorageIdentifier{Uuid: usage.FsID.Mountpoint}
		}

		if usage.UsedBytes != nil {
			element.UsedBytes = &pb.UInt64Value{Value: usage.UsedBytes.Value}
		}

		if usage.InodesUsed != nil {
			element.InodesUsed = &pb.UInt64Value{Value: usage.InodesUsed.Value}
		}

		pbFsUsage = append(pbFsUsage, element)
	}

	return
}

// ImageFSInfo returns information of the filesystem that is used to store images.
func (s *grpcImageService) ImageFsInfo(context.Context, *pb.ImageFsInfoRequest) (*pb.ImageFsInfoResponse, error) {
	fsUsage, err := imageFsinfo(s.gopts)
	if err != nil {
		return &pb.ImageFsInfoResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ImageFsInfoResponse{ImageFilesystems: copyImageFsUsage(fsUsage)}, nil
}

func storageOptsArrayToMap(storageOpts []string) (map[string]string, error) {
	if storageOpts == nil {
		return nil, nil
	}

	opts := make(map[string]string)

	for _, kvStr := range storageOpts {
		kv := strings.Split(kvStr, "=")
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("Invalid storage option %v", kvStr)
		}

		opts[kv[0]] = kv[1]
	}

	return opts, nil
}

// isulad image services
// create rootfs for container
func (s *grpcImageService) ContainerPrepare(ctx context.Context, req *pb.ContainerPrepareRequest) (*pb.ContainerPrepareResponse, error) {
	if req == nil || req.Image == "" || req.Id == "" || req.Name == "" {
		err := errors.New("Lack infomation for container prepare")
		return &pb.ContainerPrepareResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	sopts, err := storageOptsArrayToMap(req.StorageOpts)
	if err != nil {
		return &pb.ContainerPrepareResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	mountPoint, imageConfig, err := containerPrepare(s.gopts, sopts, req.Image, req.Id, req.Name)
	if err != nil {
		return &pb.ContainerPrepareResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	jsonConfig, err := json.Marshal(imageConfig)
	if err != nil {
		return &pb.ContainerPrepareResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ContainerPrepareResponse{
		MountPoint: mountPoint,
		ImageConf:  string(jsonConfig),
	}, nil
}

// remove rootfs of container
func (s *grpcImageService) ContainerRemove(ctx context.Context, req *pb.ContainerRemoveRequest) (*pb.ContainerRemoveResponse, error) {
	if req == nil || req.NameId == "" {
		err := errors.New("Lack infomation for container remove")
		return &pb.ContainerRemoveResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	err := containerRemove(s.gopts, req.NameId)
	if err != nil {
		return &pb.ContainerRemoveResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ContainerRemoveResponse{}, err
}

// mount rwlayer for container
func (s *grpcImageService) ContainerMount(ctx context.Context, req *pb.ContainerMountRequest) (*pb.ContainerMountResponse, error) {
	if req == nil || req.NameId == "" {
		err := errors.New("Lack infomation for container mount")
		return &pb.ContainerMountResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	err := containerMount(s.gopts, req.NameId)
	if err != nil {
		return &pb.ContainerMountResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ContainerMountResponse{}, err
}

// umount rwlayer of container
func (s *grpcImageService) ContainerUmount(ctx context.Context, req *pb.ContainerUmountRequest) (*pb.ContainerUmountResponse, error) {
	if req == nil || req.NameId == "" {
		err := errors.New("Lack infomation for container umount")
		return &pb.ContainerUmountResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	err := containerUmount(s.gopts, req.NameId, req.Force)
	if err != nil {
		return &pb.ContainerUmountResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ContainerUmountResponse{}, err
}

// export container rootfs
func (s *grpcImageService) ContainerExport(ctx context.Context, req *pb.ContainerExportRequest) (*pb.ContainerExportResponse, error) {
	if req == nil || req.NameId == "" || req.Output == "" {
		err := errors.New("Lack infomation for export container rootfs")
		return &pb.ContainerExportResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	err := exportRootfs(s.gopts, &exportOptions{file: req.Output}, req.NameId)
	if err != nil {
		return &pb.ContainerExportResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ContainerExportResponse{}, nil
}

// get filesystem usage of container
func (s *grpcImageService) ContainerFsUsage(ctx context.Context, req *pb.ContainerFsUsageRequest) (*pb.ContainerFsUsageResponse, error) {
	if req == nil || req.NameId == "" {
		err := errors.New("Lack infomation for container filesystem usage")
		return &pb.ContainerFsUsageResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	fsUsage, err := containerFilesystemUsage(s.gopts, req.NameId)
	if err != nil {
		return &pb.ContainerFsUsageResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ContainerFsUsageResponse{Usage: string(fsUsage)}, nil
}

// get status of graphdriver
func (s *grpcImageService) GraphdriverStatus(ctx context.Context, req *pb.GraphdriverStatusRequest) (*pb.GraphdriverStatusResponse, error) {
	status, err := storageStatus(s.gopts)
	if err != nil {
		return &pb.GraphdriverStatusResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	var gotBackingFs bool
	resp := &pb.GraphdriverStatusResponse{}
	for _, kv := range status {
		if kv[0] == "Backing Filesystem" {
			gotBackingFs = true
		}
		resp.Status += fmt.Sprintf("%s: %s\n", kv[0], kv[1])
	}

	if !gotBackingFs {
		err := errors.New("Internal error, failed to get backing filesystem")
		return &pb.GraphdriverStatusResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return resp, nil
}

// get metadata of graphdriver
func (s *grpcImageService) GraphdriverMetadata(ctx context.Context, req *pb.GraphdriverMetadataRequest) (*pb.GraphdriverMetadataResponse, error) {
	var err error

	if req == nil || req.NameId == "" {
		err = errors.New("Lack infomation for driver metadata")
		return &pb.GraphdriverMetadataResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	resp := &pb.GraphdriverMetadataResponse{}
	resp.Metadata, resp.Name, err = storageMetadata(s.gopts, req.NameId)
	if err != nil {
		return &pb.GraphdriverMetadataResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return resp, nil
}

// login registry
func (s *grpcImageService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req == nil || req.Server == "" || req.Username == "" || req.Password == "" {
		err := errors.New("Lack infomation for login")
		return &pb.LoginResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	sys := &types.SystemContext{
		DockerInsecureSkipTLSVerify:       types.NewOptionalBool(!s.gopts.TLSVerify),
		DockerDaemonInsecureSkipTLSVerify: !s.gopts.TLSVerify,
		AuthFilePath:                      defaultAuthFilePath(),
	}

	err := loginRegistry(s.gopts, sys, req.Username, req.Password, req.Server)
	if err != nil {
		return &pb.LoginResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.LoginResponse{}, nil
}

// logout registry
func (s *grpcImageService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if req == nil || req.Server == "" {
		err := errors.New("Lack infomation for logout")
		return &pb.LogoutResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	sys := &types.SystemContext{
		DockerInsecureSkipTLSVerify:       types.NewOptionalBool(!s.gopts.TLSVerify),
		DockerDaemonInsecureSkipTLSVerify: !s.gopts.TLSVerify,
		AuthFilePath:                      defaultAuthFilePath(),
	}

	err := logoutRegistry(sys, req.Server)
	if err != nil {
		return &pb.LogoutResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.LogoutResponse{}, nil
}

// health check service
func (s *grpcImageService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{}, nil
}

// list containers
func (s *grpcImageService) ListContainers(ctx context.Context, req *pb.ListContainersRequest) (*pb.ListContainersResponse, error) {
	containers, err := containerList(s.gopts)
	if err != nil {
		return &pb.ListContainersResponse{
			Errmsg: err.Error(),
			Cc:     1,
		}, err
	}

	return &pb.ListContainersResponse{Containers: containers}, nil
}
