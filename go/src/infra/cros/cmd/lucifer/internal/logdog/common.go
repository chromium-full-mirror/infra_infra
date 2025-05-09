// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logdog

import "log"

// printer implements the printing part of the Logger interface.
type printer struct {
	logger *log.Logger
}

// Print implements the Logger interface.
func (p printer) Print(v ...any) {
	p.logger.Print(v...)
}

// Printf implements the Logger interface.
func (p printer) Printf(format string, v ...any) {
	p.logger.Printf(format, v...)
}

// Println implements the Logger interface.
func (p printer) Println(v ...any) {
	p.logger.Println(v...)
}
