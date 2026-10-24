package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"elt/internal/apiclient"
)

type connFlags struct {
	addr        string
	dataDir     string
	projectName string
	token       string
	insecure    bool
}

func addConnFlags(fs *flag.FlagSet) *connFlags {
	cf := &connFlags{}
	fs.StringVar(&cf.addr, "addr", "", "host:port (or scheme://host[:port], e.g. https://... for a TLS-terminated server) of a running `elt serve` to talk to, instead of a local project directory")
	fs.StringVar(&cf.dataDir, "data-dir", "", "local project directory (used when --addr is not given)")
	fs.StringVar(&cf.projectName, "project", "New Project", "project name; only used the first time --data-dir is initialized")
	fs.StringVar(&cf.token, "token", "", "bearer token to authenticate with a remote --addr server that has users configured; ignored with --data-dir")
	fs.BoolVar(&cf.insecure, "insecure", false, "skip TLS certificate verification for a https:// --addr — needed for elt's own self-signed certificate (store's EnsureSelfSignedCert has no real CA behind it); ignored with --data-dir or a plain http:// --addr")
	return cf
}

func (cf *connFlags) connect() (baseURL string, cleanup func(), err error) {
	client, cleanup, err := apiclient.Connect(cf.addr, cf.dataDir, cf.projectName, cf.token, cf.insecure)
	if err != nil {
		return "", nil, err
	}
	currentToken = client.Token
	currentHTTPClient = client.HTTPClient
	return client.BaseURL, cleanup, nil
}

var currentToken string

var currentHTTPClient *http.Client

func apiCall(method, baseURL, path string, body []byte) ([]byte, error) {
	return (&apiclient.Client{BaseURL: baseURL, Token: currentToken, HTTPClient: currentHTTPClient}).Call(method, path, body)
}

func apiCallJSON(method, baseURL, path string, reqBody any, out any) error {
	return (&apiclient.Client{BaseURL: baseURL, Token: currentToken, HTTPClient: currentHTTPClient}).CallJSON(method, path, reqBody, out)
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b)
}
