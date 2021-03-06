// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package cmd

import "github.com/spf13/cobra"

var (
	hostCmd = &cobra.Command{
		Use:   "host",
		Short: "Control host list",
	}
	hostAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add new hosts",
		Run:   sendCmd,
		Args:  cobra.MinimumNArgs(1),
	}
	hostRmCmd = &cobra.Command{
		Use:   "rm",
		Short: "Remove hosts",
		Run:   sendCmd,
		Args:  cobra.MinimumNArgs(1),
	}
)

func init() {
	RootCmd.AddCommand(hostCmd)
	hostCmd.AddCommand(hostAddCmd)
	hostCmd.AddCommand(hostRmCmd)
}

// The MIT License (MIT)
//
// Copyright © 2016-2017 Martin Tournoij
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// The software is provided "as is", without warranty of any kind, express or
// implied, including but not limited to the warranties of merchantability,
// fitness for a particular purpose and noninfringement. In no event shall the
// authors or copyright holders be liable for any claim, damages or other
// liability, whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or other dealings
// in the software.
