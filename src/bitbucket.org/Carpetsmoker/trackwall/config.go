// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Parse the configuration file.
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	urlParser "net/url"
	"os"
	"os/user"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/Carpetsmoker/sconfig"
)

// ConfigT holds the config
type ConfigT struct {
	ControlListen *AddrT
	DNSListen     *AddrT
	DNSForward    *AddrT
	HTTPListen    *AddrT
	HTTPSListen   *AddrT
	RootCert      string
	RootKey       string
	User          *UserT
	Chroot        string
	CacheHosts    int
	CacheDNS      int
	Color         bool

	// A list of the various sources
	Hostlists     [][]string
	Unhostlists   [][]string
	Regexplists   [][]string
	Unregexplists [][]string

	Hosts     []string
	Unhosts   []string
	Regexps   []string
	Unregexps []string

	Surrogates [][]string

	// Lock the config file when loading/reloading. None of the global _*
	// variables (_config, _hosts, etc.) should be accessed when this is true
	// since not all data may be properly loaded.
	Locked bool
}

// AddrT is an IP or hostname
type AddrT struct {
	Host string
	Port int
}

// UserT is a system user
type UserT struct {
	user.User

	// the user.User.{Uid,Gid} are strings, not ints :-/
	UID int
	GID int
}

// loadConfig will load a config file from path
func loadConfig(path string) error {
	sconfig.TypeHandlers["*regexp.Regexp"] = func(field *reflect.Value, v []string) interface{} {
		return regexp.MustCompile(v[0])
	}
	sconfig.TypeHandlers["*main.AddrT"] = func(field *reflect.Value, v []string) interface{} {
		a := &AddrT{}
		a.set(v[0])
		return a
	}
	sconfig.TypeHandlers["*main.UserT"] = func(field *reflect.Value, v []string) interface{} {
		u := &UserT{}
		//err := u.set(value.(string))
		//if err != nil {
		//	return u, false
		//}
		u.set(v[0])
		return u
	}

	return sconfig.Parse(&_config, path, sconfig.Handlers{
		// Handler for specific keys, rather than types
		"CacheDNS": func(l []string) {
			//c2.CacheDNS = durationToSeconds(one(splitline))
			_config.CacheDNS, _ = durationToSeconds(l[0])
			//fmt.Printf("handler -> %#v\n", config.CacheDNS)
		},
		"CacheHosts": func(l []string) {
			//c2.CacheDNS = durationToSeconds(one(splitline))
			_config.CacheHosts, _ = durationToSeconds(l[0])
			//fmt.Printf("handler -> %#v\n", config.CacheHosts)
		},
		"Hostlists": func(l []string) {
			// First is the type, after this 1+ urls
			for _, v := range l[1:] {
				_config.Hostlists = append(_config.Hostlists, []string{l[0], v})
			}
		},
		"Unhostlists": func(l []string) {
			// First is the type, after this 1+ urls
			for _, v := range l[1:] {
				_config.Unhostlists = append(_config.Unhostlists, []string{l[0], v})
			}
		},
		"Hosts": func(l []string) {
			for _, v := range l {
				_config.Hosts = append(_config.Hosts, v)
			}
		},
		"Unhosts": func(l []string) {
			for _, v := range l {
				_config.Unhosts = append(_config.Unhosts, v)
			}
		},
		"Regexps": func(l []string) {
			for _, v := range l {
				_config.Regexps = append(_config.Regexps, v)
			}
		},
		"Unregexps": func(l []string) {
			for _, v := range l {
				_config.Unregexps = append(_config.Unregexps, v)
			}
		},
		"Surrogates": func(l []string) {
			_config.Surrogates = append(_config.Surrogates, []string{l[0], strings.Join(l[1:], " ")})
		},
	})
}

// Get it as a string: host:port
func (a *AddrT) String() string {
	return fmt.Sprintf("%v:%v", a.Host, a.Port)
}

// Set it from a host:port string.
func (a *AddrT) set(addr string) {
	// TODO: Not ipv6 safe
	if strings.Index(addr, ":") < 0 {
		addr += ":53"
	}

	host, port, err := net.SplitHostPort(addr)
	fatal(err)
	a.Host = host
	a.Port, err = strconv.Atoi(port)
	fatal(err)
}

// Set it from a username.
func (u *UserT) set(username string) {
	user, err := user.Lookup(username)
	fatal(err)
	u.User = *user

	u.UID, err = strconv.Atoi(user.Uid)
	fatal(err)

	u.GID, err = strconv.Atoi(user.Gid)
	fatal(err)
}

// Check if name is in any of the *lists
func (s *ConfigT) hasDomain(name string) bool {
	check := func(arr [][]string) bool {
		for _, v := range arr {
			purl, _ := urlParser.Parse(v[1])
			if purl.Host == name {
				return true
			}
		}
		return false
	}

	return check(s.Hostlists) || check(s.Unhostlists) ||
		check(s.Regexplists) || check(s.Unregexplists)
}

// Read the hosts information *after* starting the DNS server because we can add
// hosts from remote sources (and thus needs DNS)
func (s *ConfigT) readHosts() {
	stat, err := os.Stat("/cache/compiled")

	if err == nil {
		expires := stat.ModTime().Add(time.Duration(_config.CacheHosts) * time.Second)
		if time.Now().Unix() > expires.Unix() {
			warn(fmt.Errorf("the compiled list has expired, not using it"))
		} else {
			info("using the compiled list")
			fp, err := os.Open("/cache/compiled")
			fatal(err)
			defer func() { _ = fp.Close() }()

			scanner := bufio.NewScanner(fp)
			for scanner.Scan() {
				s.addHost(scanner.Text())
			}
			return
		}
	}

	for _, v := range s.Hostlists {
		s.loadList(v[0], v[1], s.addHost)
	}
	for _, v := range s.Unhostlists {
		s.loadList(v[0], v[1], s.removeHost)
	}
	for _, v := range s.Regexplists {
		s.loadList(v[0], v[1], s.addRegexp)
	}
	for _, v := range s.Unregexplists {
		s.loadList(v[0], v[1], s.removeRegexp)
	}

	for _, v := range s.Hosts {
		s.addHost(v)
	}
	for _, v := range s.Unhosts {
		s.removeHost(v)
	}
	for _, v := range s.Regexps {
		s.addRegexp(v)
	}
	for _, v := range s.Unregexps {
		s.removeRegexp(v)
	}

	for _, v := range s.Surrogates {
		s.compileSurrogate(v[0], v[1])
	}
}

// Add host to _hosts
func (s *ConfigT) addHost(name string) {
	// Remove www.
	if strings.HasPrefix(name, "www.") {
		name = strings.Replace(name, "www.", "", 1)
	}

	// TODO: For some reason this happens sometimes. Find the source and fix
	// properly.
	if name == "" {
		return
	}

	// We already got this
	if _, has := _hosts[name]; has {
		return
	}

	_hosts[name] = ""
}

// Compile all the sources in one file, saves some memory and makes lookups a
// bit faster
func (s *ConfigT) compile() {
	newHosts := make(map[string]string)

outer:
	for name := range _hosts {
		labels := strings.Split(name, ".")

		// This catches adding "s8.addthis.com" while "addthis.com" is in the list
		c := ""
		l := len(labels)
		for i := 0; i < l; i++ {
			if c == "" {
				c = labels[l-i-1]
			} else {
				c = labels[l-i-1] + "." + c
			}

			_, have := newHosts[c]
			if have {
				continue outer
			}
		}

		// This catches adding "addthis.com" while "s7.addthis.com" is in the list;
		// in which case we want to remove the former.
		for host := range newHosts {
			if strings.HasSuffix(host, name) {
				delete(newHosts, name)
			}
		}

		newHosts[name] = ""
	}

	fp, err := os.Create("/cache/compiled")
	fatal(err)
	defer func() { _ = fp.Close() }()
	for k := range newHosts {
		_, err = fp.WriteString(fmt.Sprintf("%v\n", k))
		fatal(err)
	}

	fmt.Printf("Compiled %v hosts to %v entries\n", len(_hosts), len(newHosts))
}

// Remove host from _hosts
func (s *ConfigT) removeHost(v string) {
	delete(_hosts, v)
}

// Add regexp to _regexpx
func (s *ConfigT) addRegexp(v string) {
	c, err := regexp.Compile(v)
	fatal(err)
	_regexps = append(_regexps, c)
}

// Remove regexp to _regexpx
func (s *ConfigT) removeRegexp(v string) {
	for i, r := range _regexps {
		if r.String() == v {
			_regexps = append(_regexps[:i], _regexps[i+1:]...)
			return
		}
	}
}

// Load a list and execute cb() on every item we find.
// TODO: Add option to restrict format (e.g. regexplist hosts ... shouldn't be
// allowed).
// TODO: Allow loading remote config files in the trackwall format (which only
// parses host, hostlist, etc. and *not* dns-listen and such).
func (s *ConfigT) loadList(format string, url string, cb func(line string)) {
	fp, err := s.loadCachedURL(url)
	fatal(err)
	defer func() { _ = fp.Close() }()
	scanner := bufio.NewScanner(fp)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if format == "hosts" {
			if line[0] == '#' {
				continue
			}
			// Remove everything before the first space and after the first #
			line = strings.Join(strings.Split(line, " ")[1:], " ")
			line = strings.Split(line, "#")[0]
			line = strings.TrimSpace(line)

			// Some sites also add this to the hosts file they offer, which is
			// not wanted for us
			if line == "localhost" || line == "localhost.localdomain" || line == "broadcasthost" || line == "local" {
				continue
			}
		} else if format == "plain" {
			// Nothing needed
		} else {
			fatal(fmt.Errorf("unknown format: %v\n", format))
		}

		cb(line)
	}
}

// Load URL with cache.
func (s *ConfigT) loadCachedURL(url string) (*os.File, error) {
	// Load from filesystem
	if strings.HasPrefix(url, "file://") {
		return os.Open(url[7:])
	}

	// TODO: Check error (e.g. perm. denied)
	err := os.MkdirAll("/cache/hosts", 0755)
	fatal(err)
	cachename := "/cache/hosts/" + regexp.MustCompile(`\W+`).ReplaceAllString(url, "-")

	stat, err := os.Stat(cachename)
	if err != nil && !os.IsNotExist(err) {
		fatal(err)
	}

	// Check if cache expires
	if stat != nil {
		expires := stat.ModTime().Add(time.Duration(_config.CacheHosts) * time.Second)
		if time.Now().Unix() > expires.Unix() {
			stat = nil
			os.Remove(cachename)
		}
	}

	// Download
	if stat == nil {
		info("downloading " + url)
		resp, err := http.Get(url)
		if resp != nil {
			defer resp.Body.Close()
		}
		fatal(err)

		fp, err := os.Create(cachename)
		fatal(err)
		data, err := ioutil.ReadAll(resp.Body)
		fp.Write(data)
		_ = fp.Close()
	}

	return os.Open(cachename)
}

type surrogateT struct {
	*regexp.Regexp
	script string
}

// "Compile" a surrogate into the config.hosts array. This uses a bit more memory,
// but saves a lot of regexp checks later.
func (s *ConfigT) compileSurrogate(reg string, sur string) {
	sur = strings.Replace(sur, "@@", "function(){}", -1)
	//info(fmt.Sprintf("compiling surrogate %s -> %s", reg, sur[:40]))

	c, err := regexp.Compile(reg)

	xx := surrogateT{c, sur}
	_surrogatesLock.Lock()
	_surrogates = append(_surrogates, xx)
	_surrogatesLock.Unlock()

	fatal(err)

	found := 0
	for host := range _hosts {
		if c.MatchString(host) {
			found++
			//info(fmt.Sprintf("  adding for %s", host))
			_hosts[host] = sur
		}
	}

	if found > 50 {
		warn(fmt.Errorf("the surrogate %s matches %d hosts. Are you sure this is correct?",
			reg, found))
	}
}

func findResolver() (string, error) {
	fp, err := os.Open("/etc/resolv.conf")
	fatal(err)

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "nameserver") {
			continue
		}
		if strings.HasSuffix(line, _config.DNSListen.Host) {
			continue
		}

		return line[strings.LastIndex(line, " ")+1:] + ":53", nil
	}

	return "", fmt.Errorf("unable to find host in /etc/resolv.conf")
}

// The MIT License (MIT)
//
// Copyright © 2016 Martin Tournoij
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
