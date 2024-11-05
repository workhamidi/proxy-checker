package main


import (
	"net/url"
)

type GeoNodeResponse struct {
	Data []GeoNodeProxy `json:"data"`
}

type GeoNodeProxy struct {
	IP        string   `json:"ip"`
	Port      string   `json:"port"`
	Protocols []string `json:"protocols"`
}

type ProxyscrapeResponse struct {
	Data []ProxyScrapeProxy `json:"proxies"`
}

type ProxyScrapeProxy struct {
	Protocol string `json:"protocol"`
	Proxy    string `json:"proxy"`
}

type ProxyTask struct {
	ProxyUrl  *url.URL
	index string
}
