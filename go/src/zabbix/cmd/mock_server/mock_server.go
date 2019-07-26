/*
** Zabbix
** Copyright (C) 2001-2019 Zabbix SIA
**
** This program is free software; you can redistribute it and/or modify
** it under the terms of the GNU General Public License as published by
** the Free Software Foundation; either version 2 of the License, or
** (at your option) any later version.
**
** This program is distributed in the hope that it will be useful,
** but WITHOUT ANY WARRANTY; without even the implied warranty of
** MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
** GNU General Public License for more details.
**
** You should have received a copy of the GNU General Public License
** along with this program; if not, write to the Free Software
** Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
**/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"
	"zabbix/pkg/comms"
	"zabbix/pkg/conf"
	"zabbix/pkg/log"
)

type MockServerOptions struct {
	LogType          string `conf:",,,console"`
	LogFile          string `conf:",optional"`
	DebugLevel       int    `conf:",,0:5,3"`
	Port             int    `conf:",,1:65535,10051"`
	Timeout          int    `conf:",,1:30,5"`
	ActiveChecksFile string `conf:",optional"`
}

var options MockServerOptions

func handleConnection(c comms.ZbxConnection, activeChecks []byte, tFlag int) {
	defer c.Close()

	js, err := c.Read(time.Second * time.Duration(tFlag))
	if err != nil {
		log.Warningf("Read failed: %s\n", err)
		return
	}

	log.Debugf("got '%s'", string(js))

	var pairs map[string]interface{}
	if err := json.Unmarshal(js, &pairs); err != nil {
		log.Warningf("Unmarshal failed: %s\n", err)
		return
	}

	switch pairs["request"] {
	case "active checks":
		err = c.Write(activeChecks, time.Second*time.Duration(tFlag))
		if err != nil {
			log.Warningf("Write failed: %s\n", err)
			return
		}
	default:
		log.Warningf("Unsupported request: %s\n", pairs["request"])
		return
	}

}

func main() {
	var confFlag string
	const (
		confDefault     = "mock_server.conf"
		confDescription = "Path to the configuration file"
	)
	flag.StringVar(&confFlag, "config", confDefault, confDescription)
	flag.StringVar(&confFlag, "c", confDefault, confDescription+" (shorhand)")

	var foregroundFlag bool
	const (
		foregroundDefault     = true
		foregroundDescription = "Run Zabbix agent in foreground"
	)
	flag.BoolVar(&foregroundFlag, "foreground", foregroundDefault, foregroundDescription)
	flag.BoolVar(&foregroundFlag, "f", foregroundDefault, foregroundDescription+" (shorhand)")
	flag.Parse()

	if err := conf.Load(confFlag, &options); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	if err := conf.Load(confFlag, &options); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	var logType, logLevel int
	switch options.LogType {
	case "console":
		logType = log.Console
	case "file":
		logType = log.File
	}
	switch options.DebugLevel {
	case 0:
		logLevel = log.Info
	case 1:
		logLevel = log.Crit
	case 2:
		logLevel = log.Err
	case 3:
		logLevel = log.Warning
	case 4:
		logLevel = log.Debug
	case 5:
		logLevel = log.Trace
	}

	if err := log.Open(logType, logLevel, options.LogFile); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot initialize logger: %s\n", err.Error())
		os.Exit(1)
	}

	greeting := fmt.Sprintf("Starting Zabbix Agent [(hostname placeholder)]. (version placeholder)")
	log.Infof(greeting)

	if foregroundFlag {
		if options.LogType != "console" {
			fmt.Println(greeting)
		}
		fmt.Println("Press Ctrl+C to exit.")
	}

	log.Infof("using configuration file: %s", confFlag)

	activeChecks, err := ioutil.ReadFile(options.ActiveChecksFile)
	if err != nil {
		log.Critf("Cannot read active checks file: %s\n", err)
		return
	}

	var ln comms.ZbxListener

	err = ln.Listen(":" + strconv.Itoa(options.Port))
	if err != nil {
		log.Critf("Listen failed: %s\n", err)
		return
	}
	defer ln.Close()

	for {
		var c comms.ZbxConnection

		err = c.Accept(&ln)
		if err != nil {
			log.Critf("Accept failed: %s\n", err)
			return
		}

		go handleConnection(c, activeChecks, options.Timeout)
	}
}
