package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	 "strconv"


	"github.com/corpix/uarand"
	"github.com/schollz/progressbar/v3"
)

func checkProxy(parameters interface{}, channel chan<- string, bar *progressbar.ProgressBar, silentMode bool) {
	temp := parameters.(ProxyTask)

	logDebugf("Checking proxy %s", temp.ProxyUrl.String())
	
	if silentMode == false{
		bar.Add(1)
	}

	ctx, cncl := context.WithTimeout(context.Background(), time.Duration(*ProxyRequestTimeout)*time.Second)
	defer cncl()

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(temp.ProxyUrl),
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://google.com", nil)
	if err != nil {
		logErrorf("%s Error creating request: %s", temp.index, err.Error())
		return
	}

	req.Header.Add("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		logErrorf("%s Proxy %s is not working: %s", temp.index, temp.ProxyUrl.String(), err.Error())
		return
	}
	defer resp.Body.Close()

	logDebugf("Received response with status code %d", resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		channel <- temp.ProxyUrl.String()
		if silentMode == false{
			bar.Describe(strconv.Itoa(len(channel)))
		}	    
	} else {
		logErrorf("%s Proxy %s is not working: Received non-200 response: %d", temp.index, temp.ProxyUrl.String(), resp.StatusCode)
		return
	}
}

func getProxies(url string, channel chan<- []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	logInfof("Try to retrieve proxies from URL %s", url)

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logErrorf("Error creating request: %s", err.Error())
		return
	}

	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		logErrorf("Error during request: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logErrorf("Error reading response body: %s", err.Error())
		return
	}

	proxyCount := len(strings.Split(string(body), "\n")) - 1
	logInfof("%d proxies have been received from URL %s", proxyCount, url)

	channel <- body
}

func InitProxiesSlice(urls []string) []string {

	var wg sync.WaitGroup
	channel := make(chan []byte)

	allProxies := make([]string, 0)

	for _, url := range urls {
		wg.Add(1)
		go getProxies(url, channel, &wg)
	}

	go func() {
		wg.Wait()
		close(channel)
	}()

	for proxiesSlice := range channel {
		allProxies = append(allProxies, strings.Split(string(proxiesSlice), "\n")...)
	}

	logInfof("Total proxies collected: %d", len(allProxies))

	return allProxies
}

func fetchGeoNodeProxies(protocol string) ([]string, error) {
	var allProxies []string
	url := fmt.Sprintf("https://proxylist.geonode.com/api/proxy-list?protocols=%s", protocol)
	page := 0

	for {
		logInfof("Fetching proxies from GeoNode, page %d", page)

		resp, err := http.Get(fmt.Sprintf("%s&limit=500&page=%d", url, page))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch data from GeoNode: %s", resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var response GeoNodeResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, err
		}

		if len(response.Data) == 0 {
			break
		}

		for _, proxy := range response.Data {
			proxyString := fmt.Sprintf("%s://%s:%s", protocol, proxy.IP, proxy.Port)
			allProxies = append(allProxies, proxyString)
		}

		logInfof("Retrieved %d proxies from GeoNode, page %d", len(response.Data), page)
		page++
	}
	return allProxies, nil
}

func fetchProxyScrapeProxies(protocol string) ([]string, error) {
	var allProxies []string
	url := fmt.Sprintf("https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=%s&format=json", protocol)

	logInfof("Fetching proxies from ProxyScrape")

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch data from ProxyScrape: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var proxyScrapeResponse ProxyscrapeResponse
	if err := json.Unmarshal(body, &proxyScrapeResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ProxyScrape response: %v", err)
	}

	for _, proxy := range proxyScrapeResponse.Data {
		proxyString := fmt.Sprintf("%s://%s", proxy.Protocol, proxy.Proxy)
		allProxies = append(allProxies, proxyString)
	}

	logInfof("Retrieved %d proxies from ProxyScrape", len(allProxies))

	return allProxies, nil
}

func FetchProxiesFromSites(protocol string) ([]string, error) {
	geonodeProxies, err := fetchGeoNodeProxies(protocol)
	if err != nil {
		logErrorf("Error fetching from GeoNode: %v", err)
		return nil, err
	}

	proxyscrapeProxies, err := fetchProxyScrapeProxies(protocol)
	if err != nil {
		logErrorf("Error fetching from ProxyScrape: %v", err)
		return nil, err
	}

	allProxies := append(geonodeProxies, proxyscrapeProxies...)
	logInfof("Fetched %d total proxies from GeoNode and ProxyScrape", len(allProxies))

	return allProxies, nil
}
