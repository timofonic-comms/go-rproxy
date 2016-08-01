// Copyright 2016, Cong Ding. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// author: Cong Ding <dinggnu@gmail.com>

package rproxy

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"
)

type RProxy struct {
	listenProto  string
	listenAddr   string
	backendProto string
	backendAddr  string
	rootCert     string
	serverCert   string
	serverKey    string
	clientCert   string
	clientKey    string
}

func NewRProxy(listenProto, listenAddr, backendProto, backendAddr, rootCert, serverCert, serverKey, clientCert, clientKey string) *RProxy {
	return &RProxy{
		listenProto:  strings.ToLower(listenProto),
		listenAddr:   strings.ToLower(listenAddr),
		backendProto: strings.ToLower(backendProto),
		backendAddr:  strings.ToLower(backendAddr),
		rootCert:     rootCert,
		serverCert:   serverCert,
		serverKey:    serverKey,
		clientCert:   clientCert,
		clientKey:    clientKey,
	}
}

func (rp *RProxy) Start() {
	switch rp.listenProto {
	case "tcp":
		rp.startTCP()
	case "tls":
		rp.startTLS()
	default:
		panic("listen protocol not supported")
	}
}

func (rp *RProxy) serve(conn net.Conn) error {
	switch rp.backendProto {
	case "tcp":
		rp.serveTCP(conn)
	case "tls":
		rp.serveTLS(conn)
	default:
		panic("backend protocol not supported")
	}
	return nil
}

func (rp *RProxy) startTCP() {
	// Resolve network address
	lAddr, err := net.ResolveTCPAddr("tcp", rp.listenAddr)
	if err != nil {
		panic(err)
	}
	// Listen to TCP connections
	ln, err := net.ListenTCP("tcp", lAddr)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	// Handle connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error (%v)\n", err)
			continue
		}
		go rp.serve(conn)
	}
}

func (rp *RProxy) startTLS() {
	// Load root pem
	rootPEM, err := ioutil.ReadFile(rp.rootCert)
	if err != nil {
		panic("failed to load root certificate")
	}
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(rootPEM)); !ok {
		panic("failed to parse root certificate")
	}
	// Load server pem
	cert, err := tls.LoadX509KeyPair(rp.serverCert, rp.serverKey)
	if err != nil {
		log.Fatalf("failed to load server tls certificate: %s", err)
	}
	// Set config for TLS listener
	config := tls.Config{
		ClientCAs:    roots,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
	}
	// Listen to TLS connections
	ln, err := tls.Listen("tcp", rp.listenAddr, &config)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	// Handle connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error (%v)\n", err)
			continue
		}
		go rp.serve(conn)
	}
}

func (rp *RProxy) serveTCP(listenConn net.Conn) error {
	// Dial to the backend server
	backendConn, err := net.DialTimeout("tcp", rp.backendAddr, 30*time.Second)
	if err != nil {
		listenConn.Close()
		return err
	}
	// Copy network traffic from the listen connection to backend connection
	go func() {
		io.Copy(backendConn, listenConn)
		backendConn.Close()
		listenConn.Close()
	}()
	// Copy network traffic from the backend connection to listen connection
	io.Copy(listenConn, backendConn)
	backendConn.Close()
	listenConn.Close()
	return nil
}

func (rp *RProxy) serveTLS(listenConn net.Conn) error {
	// Load root pem
	rootPEM, err := ioutil.ReadFile(rp.rootCert)
	if err != nil {
		log.Fatalf("failed to load root certificate")
	}
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(rootPEM)); !ok {
		panic("failed to parse root certificate")
	}

	// Load client pem
	cert, err := tls.LoadX509KeyPair(rp.clientCert, rp.clientKey)
	if err != nil {
		panic("failed to load client tls certificate")
	}
	// Set config for TLS connections
	config := tls.Config{
		RootCAs:      roots,
		ServerName:   "testapp-server",
		Certificates: []tls.Certificate{cert},
	}
	// Dial to the beckend server
	backendConn, err := tls.Dial("tcp", rp.backendAddr, &config)
	if err != nil {
		listenConn.Close()
		return err
	}
	// Copy network traffic from the listen connection to backend connection
	go func() {
		io.Copy(backendConn, listenConn)
		backendConn.Close()
		listenConn.Close()
	}()
	// Copy network traffic from the backend connection to listen connection
	io.Copy(listenConn, backendConn)
	backendConn.Close()
	listenConn.Close()
	return nil
}