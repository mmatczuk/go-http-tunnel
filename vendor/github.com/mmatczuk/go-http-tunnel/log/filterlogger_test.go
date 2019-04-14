// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/mmatczuk/go-http-tunnel/tunnelmock"
)

func TestFilterLogger_Log(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	f := NewFilterLogger(b, 2)
	b.EXPECT().Log("level", 0)
	f.Log("level", 0)
	b.EXPECT().Log("level", 1)
	f.Log("level", 1)
	b.EXPECT().Log("level", 2)
	f.Log("level", 2)

	f.Log("level", 3)
	f.Log("level", 4)
}
