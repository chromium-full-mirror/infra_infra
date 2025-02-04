// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands

import (
	"container/list"
	"fmt"
)

func InstantiatePopFromQueue(queue *list.List, caster func(any)) (err error) {
	// Catch panics from caster.
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	if queue.Len() < 1 {
		return fmt.Errorf("queue is empty")
	}
	element := queue.Remove(queue.Front())
	caster(element)

	return nil
}
