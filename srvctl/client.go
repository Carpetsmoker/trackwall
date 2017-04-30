// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package srvctl

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"
)

// Write to the server.
func Write(what string) {
	conn, err := net.Dial("tcp", cfg.Config.ControlListen.String())
	msg.Fatal(err)
	defer func() { _ = conn.Close() }()

	fmt.Fprintf(conn, what+"\n")
	data, err := ioutil.ReadAll(conn)
	msg.Fatal(err)
	fmt.Println(strings.TrimSpace(string(data)))
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
