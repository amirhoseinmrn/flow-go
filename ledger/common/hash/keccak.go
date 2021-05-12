// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !purego && gc
// +build amd64,!purego,gc

package hash

// This function is implemented in keccakf_amd64.s.

//go:noescape

func keccakF1600(state *[25]uint64)
