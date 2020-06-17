// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmhub

type CharmInfo struct {
	Type    string
	ID      string
	Name    string
	Summary string
}

type InfoResponse struct {
	Type           string       `json:"type",yaml:"type"`
	ID             string       `json:"id",yaml:"id"`
	Name           string       `json:"name",yaml:"name"`
	Charm          Charm        `json:"charm,omitempty",yaml:"charm,omitempty"`
	ChannelMap     []ChannelMap `json:"channel-map",yaml:"channel-map,omitempty"`
	DefaultRelease ChannelMap   `json:"default-release,omitempty"`
}

type ChannelMap struct {
	Channel  Channel  `json:"channel,omitempty",yaml:"channel,omitempty"`
	Revision Revision `json:"revision,omitempty",yaml:"revision,omitempty"`
}

type Channel struct {
	Name       string   `json:"name",yaml:"name"`
	Platform   Platform `json:"platform",yaml:"platform"`
	ReleasedAt string   `json:"released-at",yaml:"released-at"`
	Risk       string   `json:"risk",yaml:"risk"`
	Track      string   `json:"track",yaml:"track"`
}

type Platform struct {
	Architecture string `json:"architecture",yaml:"architecture"`
	OS           string `json:"os",yaml:"os"`
	Series       string `json:"series",yaml:"series"`
}

type Revision struct {
	ConfigYaml   string     `json:"config-yaml",yaml:"config-yaml"`
	CreatedAt    string     `json:"created-at",yaml:"created-at"`
	Download     Download   `json:"download",yaml:"download"`
	MetadataYaml string     `json:"metadata-yaml",yaml:"metadata-yaml"`
	Platforms    []Platform `json:"platforms",yaml:"platforms"`
	Revision     int        `json:"revision",yaml:"revision"`
	Version      string     `json:"version",yaml:"version"`
}

type Download struct {
	HashSHA265 string `json:"hash-sha-265",yaml:"hash-sha-265"`
	Size       int    `json:"size",yaml:"size"`
	URL        string `json:"url",yaml:"url"`
}

type Charm struct {
	Categories  []Category `json:"categories",yaml:"categories"`
	Description string     `json:"description",yaml:"description"`
	License     string     `json:"license",yaml:"license"`
	//Media       []Media           `json:"media",yaml:"media"`
	Publisher map[string]string `json:"publisher",yaml:"publisher"`
	Summary   string            `json:"summary",yaml:"summary"`
	UsedBy    []string          `json:"used-by",yaml:"used-by"`
}

type Category struct {
	Featured bool   `json:"featured",yaml:"featured"`
	Name     string `json:"name",yaml:"name"`
}

// TODO: (hml) 2020-06-17
// Why do we fail unmarshalling to this structure?
type Media struct {
	Height int    `json:"height",yaml:"height"`
	Type   string `json:"type",yaml:"type"`
	URL    string `json:"url",yaml:"url"`
	Width  int    `json:"width",yaml:"width"`
}

type ErrorResponse struct {
	ErrorList []Error `json:"error-list",yaml:"error-list"`
}

type Error struct {
	Code    string `json:"code",yaml:"code"`
	Message string `json:"message",yaml:"message"`
}
