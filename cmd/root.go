// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/apex/log"
	"github.com/spf13/cobra"
)

var ctx *log.Logger

var logFile *os.File

// Execute is called by main.go
func Execute() {
	defer func() {
		buf := make([]byte, 1<<16)
		runtime.Stack(buf, false)
		if thePanic := recover(); thePanic != nil && ctx != nil {
			ctx.WithField("panic", thePanic).WithField("stack", string(buf)).Fatal("Stopping because of panic")
		}
	}()

	if err := BridgeCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}
