// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"reflect"
	"testing"
)

func TestLCS(t *testing.T) {
	for i, test := range []struct {
		a, b                   []byte
		indexA, indexB, length int
	}{
		{[]byte{1, 2, 3}, []byte{1, 2, 3}, 0, 0, 3},
		{[]byte{0, 1, 2, 3}, []byte{1, 2, 3, 4}, 1, 0, 3},
		{[]byte{0, 1, 2, 3, 1, 2, 3, 4, 1, 2, 3}, []byte{1, 2, 3, 4}, 4, 0, 4},
		{[]byte{1, 2, 2, 3, 4}, []byte{1, 2, 3, 4}, 2, 1, 3},
		{[]byte{0, 1, 2, 3, 4}, []byte{1, 1, 2, 2, 3, 4}, 2, 3, 3},
	} {
		ia, ib, l := longestCommonSubstring(test.a, test.b)
		if ia != test.indexA || ib != test.indexB || l != test.length {
			t.Errorf("%d: want (%d %d %d) got (%d %d %d)", i, test.indexA, test.indexB, test.length, ia, ib, l)
		}
	}
}

func TestDiff(t *testing.T) {
	for i, test := range []struct {
		a, b []byte
		d    []difference
	}{
		{
			[]byte{0, 1, 2, 3, 4},
			[]byte{1, 1, 2, 2, 3, 4},
			[]difference{
				difference{true, []byte{0}, []byte{}},
				difference{false, []byte{1}, nil},
				difference{true, []byte{}, []byte{1, 2}},
				difference{false, []byte{2, 3, 4}, nil},
			},
		},
	} {
		diffs := diffInternal(test.a, test.b)
		if !reflect.DeepEqual(diffs, test.d) {
			t.Errorf("%d want %v got %v", i, test.d, diffs)
		}
	}
}
