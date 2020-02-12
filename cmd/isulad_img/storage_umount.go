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
// Author: lifeng
// Create: 2019-05-06

package main

func storageUmount(gopts *globalOptions) error {
	store, err := getStorageStore(gopts)
	if err != nil {
		return err
	}

	driver, err := store.GraphDriver()
	if err != nil {
		return err
	}

	err = driver.Cleanup()

	return err
}
