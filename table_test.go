// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"reflect"
	"testing"
)

func TestFormatTable(t *testing.T) {
	for _, tc := range []struct {
		in   [][]string
		opts []tableOpt
		want []string
	}{
		{[][]string{}, nil, nil},
		{
			[][]string{{"ab", "foo"}, {"c", "barber"}},
			[]tableOpt{tableSpacing(2)},
			[]string{"ab  foo", "c   barber"},
		},
		{
			[][]string{{"right", "foo"}, {"really long value", "bar"}},
			[]tableOpt{tableSpacing(2), tableRightCol(0)},
			[]string{"            right  foo", "really long value  bar"},
		},
		{
			[][]string{{"first", "second"}, {"first"}},
			[]tableOpt{tableSpacing(1)},
			[]string{"first second", "first"},
		},
		{
			[][]string{{"first"}, {"first", "second"}},
			[]tableOpt{tableSpacing(1)},
			[]string{"first", "first second"},
		},
		{
			[][]string{{"", "has empty column"}, {"", "second"}},
			[]tableOpt{tableSpacing(2)},
			[]string{"has empty column", "second"},
		},
	} {
		if got := formatTable(tc.in, tc.opts...); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("formatTable(%q, ...) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
