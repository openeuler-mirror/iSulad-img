// Copyright (c) Huawei Technologies Co., Ltd. 2019. All rights reserved.
// iSulad-kit licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.
// Description: iSulad image kit
// Author: wangfengtu
// Create: 2019-09-04

package main

func containerList(gopts *globalOptions) (map[string]bool, error) {
	store, err := getStorageStore(gopts)
	if err != nil {
		return nil, err
	}

	containers, err := store.Containers()
	if err != nil {
		return nil, err
	}

	cs := make(map[string]bool)
	for _, c := range containers {
		mountCount, err := store.Mounted(c.ID)
		if err != nil {
			return nil, err
		}
		if mountCount > 0 {
			cs[c.ID] = true
		} else {
			cs[c.ID] = false
		}
	}

	return cs, err
}
