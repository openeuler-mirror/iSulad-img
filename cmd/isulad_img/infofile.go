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
// Create: 2019-07-30

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	infoFileItemPid int = iota
	infoFileItemAddr
	infoFileItemNum
	maxInfoFileSize     = 512
	maxCommSize         = 128
	daemonInstanceExist = "only one instance is allowed"
)

func createInfoFile(infoFile string, sockAddr string) error {
	return ioutil.WriteFile(infoFile, []byte(strconv.Itoa(os.Getpid())+"\n"+sockAddr), 0600)
}

func validInfoFile(infoFile string) (bool, error) {
	fi, err := os.Stat(infoFile)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	if fi.Size() > maxInfoFileSize {
		return false, fmt.Errorf("Info file have invalid size %v", fi.Size())
	}

	return false, nil
}

func readInfoFile(infoFile string) (string, string, error) {
	content, err := ioutil.ReadFile(infoFile)
	if err != nil {
		return "", "", err
	}

	items := strings.Split(string(content), "\n")
	if len(items) != infoFileItemNum {
		return "", "", fmt.Errorf("Invalid item number in info file, expected %v", infoFileItemNum)
	}

	pid, err := strconv.Atoi(items[infoFileItemPid])
	if err != nil {
		return "", "", err
	}

	return strconv.Itoa(pid), items[infoFileItemAddr], nil
}

func readCommStr(pidStr string) (bool, string, error) {
	commPath := "/proc/" + pidStr + "/comm"
	fi, err := os.Stat(commPath)
	if os.IsNotExist(err) {
		return true, "", nil
	} else if err != nil {
		return false, "", err
	}

	if fi.Size() > maxCommSize {
		return false, "", fmt.Errorf("Comm file have invalid size %v", fi.Size())
	}

	comm, err := ioutil.ReadFile(commPath)
	if err != nil {
		return false, "", err
	}

	return false, strings.TrimSuffix(string(comm), "\n"), nil
}

func isSameComm(comm, curComm string) bool {
	return comm == filepath.Base(curComm)
}

func isDaemonInstanceExist(infoFile string) (string, error) {
	nonexist, err := validInfoFile(infoFile)
	if nonexist {
		return "", os.ErrNotExist
	} else if err != nil {
		return "", err
	}

	pidStr, sockAddr, err := readInfoFile(infoFile)
	if err != nil {
		return "", err
	}

	nonexist, comm, err := readCommStr(pidStr)
	if nonexist {
		return "", os.ErrNotExist
	} else if err != nil {
		return "", err
	}

	if isSameComm(comm, os.Args[0]) {
		return sockAddr, fmt.Errorf("%v found running, "+daemonInstanceExist, comm)
	}

	return "", os.ErrNotExist
}

func newInfoFile(infoFile string, sockAddr string) error {
	_, err := isDaemonInstanceExist(infoFile)
	if err == os.ErrNotExist {
		return createInfoFile(infoFile, sockAddr)
	}

	return err
}

func delInfoFile(infoFile string) {
	if err := os.Remove(infoFile); err != nil && !os.IsNotExist(err) {
		logrus.Warnf("Remove info file failed: %v", err)
	}
}
