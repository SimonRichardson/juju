// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmhub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/juju/charmrepo/v5/csclient/params"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	httprequest "gopkg.in/httprequest.v1"
	"gopkg.in/macaroon-bakery.v2/httpbakery"

	corecharmhub "github.com/juju/juju/core/charmhub"
)

// ServerURL holds the default location of the global charm hub.
// An alternate location can be configured by changing the URL field in the
// Params struct.
// https://api.snapcraft.io/charms/v5
var (
	ServerURL = "https://api.snapcraft.io/"
)

const (
	userAgentKey   = "User-Agent"
	userAgentValue = "Golang_CSClient/4.0"
	apiVersion     = "v2"
	apiName        = "charms"
)

func NewClient(server string) (*Client, error) {
	bakeryClient := &httpbakery.Client{
		Client: httpbakery.NewHTTPClient(),
	}
	return New(Params{
		URL:          server,
		User:         "",
		Password:     "",
		BakeryClient: bakeryClient,
	}), nil
}

const defaultMinMultipartUploadSize = 5 * 1024 * 1024

// Client represents the client side of a charm store.
type Client struct {
	params                 Params
	bclient                httpClient
	header                 http.Header
	statsDisabled          bool
	channel                params.Channel
	minMultipartUploadSize int64
}

// Params holds parameters for creating a new charm store client.
type Params struct {
	// URL holds the root endpoint URL of the charmstore,
	// with no trailing slash, not including the version.
	// For example https://api.jujucharms.com/charmstore
	// If empty, the default charm store client location is used.
	URL string

	// User holds the name to authenticate as for the client. If User is empty,
	// no credentials will be sent.
	User string

	// Password holds the password for the given user, for authenticating the
	// client.
	Password string

	// BakeryClient holds the bakery client to use when making
	// requests to the store. This is used in preference to
	// HTTPClient.
	BakeryClient *httpbakery.Client
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// New returns a new charm store client.
func New(p Params) *Client {
	if p.URL == "" {
		p.URL = ServerURL
	}
	bclient := p.BakeryClient
	if bclient == nil {
		bclient = httpbakery.NewClient()
		bclient.AddInteractor(httpbakery.WebBrowserInteractor{})
	}
	return &Client{
		bclient:                bclient,
		params:                 p,
		minMultipartUploadSize: defaultMinMultipartUploadSize,
	}
}

// SetMinMultipartUploadSize sets the minimum size of resource upload
// that will trigger a multipart upload. This is mainly useful for testing.
func (c *Client) SetMinMultipartUploadSize(n int64) {
	c.minMultipartUploadSize = n
}

// ServerURL returns the charm store URL used by the client.
func (c *Client) ServerURL() string {
	return c.params.URL
}

// DisableStats disables incrementing download stats when retrieving archives
// from the charm store.
func (c *Client) DisableStats() {
	c.statsDisabled = true
}

// WithChannel returns a new client whose requests are done using the
// given channel.
func (c *Client) WithChannel(channel params.Channel) *Client {
	client := *c
	client.channel = channel
	return &client
}

// Channel returns the currently set channel.
func (c *Client) Channel() params.Channel {
	return c.channel
}

// SetHTTPHeader sets custom HTTP headers that will be sent to the charm store
// on each request.
func (c *Client) SetHTTPHeader(header http.Header) {
	c.header = header
}

// Do makes an arbitrary request to the charm store.
// It adds appropriate headers to the given HTTP request,
// sends it to the charm store, and returns the resulting
// response. Do never returns a response with a status
// that is not http.StatusOK.
//
// The URL field in the request is ignored and overwritten.
//
// This is a low level method - more specific Client methods
// should be used when possible.
//
// Note that if a body is supplied in the request, it should
// implement io.Seeker.
//
// Any error returned from the underlying httpbakery.Do
// request will have an unchanged error cause.
func (c *Client) do(req *http.Request, path string) (*http.Response, error) {
	if c.params.User != "" {
		userPass := c.params.User + ":" + c.params.Password
		authBasic := base64.StdEncoding.EncodeToString([]byte(userPass))
		req.Header.Set("Authorization", "Basic "+authBasic)
	}

	// Prepare the request.
	if !strings.HasPrefix(path, "/") {
		return nil, errors.Errorf("path %q is not absolute", path)
	}
	for k, vv := range c.header {
		req.Header[k] = append(req.Header[k], vv...)
	}

	// Set the user-agent if one isn't supplied
	if userAgent := req.Header.Get(userAgentKey); userAgent == "" {
		req.Header.Set(userAgentKey, userAgentValue)
	}

	u, err := url.Parse(c.params.URL + apiVersion + "/" + apiName + path)
	if err != nil {
		return nil, errors.Mask(err)
	}
	if c.channel != params.NoChannel {
		values := u.Query()
		values.Set("channel", string(c.channel))
		u.RawQuery = values.Encode()
	}
	req.URL = u
	logger.Criticalf("do() %s", spew.Sdump(req))

	// Send the request.
	resp, err := c.bclient.Do(req)
	if err != nil {
		return nil, errors.Annotatef(err, "failed executing the request %s", u)
	}

	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the response error.
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "cannot read response body")
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		return nil, errors.Errorf("unexpected response status from server: %v", resp.Status)
	}
	var perr params.Error
	if err := json.Unmarshal(data, &perr); err != nil {
		return nil, errors.Annotatef(err, "cannot unmarshal error response %q", sizeLimit(data))
	}

	if perr.Message == "" {
		return nil, errors.Errorf("error response with empty message %s", sizeLimit(data))
	}
	return nil, &perr
}

func sizeLimit(data []byte) []byte {
	const max = 1024
	if len(data) < max {
		return data
	}
	return append(data[0:max], fmt.Sprintf(" ... [%d bytes omitted]", len(data)-max)...)
}

// Get makes a GET request to the given path in the charm store (not
// including the host name or version prefix but including a leading /),
// parsing the result as JSON into the given result value, which should
// be a pointer to the expected data, but may be nil if no result is
// desired.
func (c *Client) get(path string, result interface{}) error {
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		return errors.Annotate(err, "cannot make new request")
	}
	resp, err := c.do(req, path)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Parse the response.
	if err := httprequest.UnmarshalJSONResponse(resp, result); err != nil {
		return errors.Annotate(err, "charm hub client get")
	}
	return nil
}

var logger = loggo.GetLogger("juju.charmhub")

//Summary: GET /v2/charms/info/<name>
//
//"Get charm info" handler.
//
//Given a name, return the details of the corresponding charm or bundle, and its released revisions.
//
//Supports the following optional query params:
//
//fields: A comma separated list of field names to include in the response. Optional. The following are always returned: type, id, name. Possible field names are in the successful response description below.
func (c *Client) Info(name string) (corecharmhub.InfoResponse, error) {
	infoURL := "/info/" + name
	logger.Criticalf("Info(%s): %s", name, infoURL)
	resp := corecharmhub.InfoResponse{}
	err := c.get(infoURL, &resp)
	logger.Criticalf("Info(%s): %s", name, spew.Sdump(resp))
	return resp, err
}
