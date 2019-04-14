// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/mmatczuk/go-http-tunnel/tunnelmock"
)

func TestContext_Log(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	b.EXPECT().Log("key", "val", "sufix", "")
	NewContext(b).With("sufix", "").Log("key", "val")

	b.EXPECT().Log("prefix", "", "key", "val")
	NewContext(b).WithPrefix("prefix", "").Log("key", "val")

	b.EXPECT().Log("prefix", "", "key", "val", "sufix", "")
	NewContext(b).With("sufix", "").WithPrefix("prefix", "").Log("key", "val")
}
