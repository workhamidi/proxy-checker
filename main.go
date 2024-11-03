package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/corpix/uarand"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

type GeoNodeResponse struct {
	Data []GeoNodeProxy `json:"data"`
}

type GeoNodeProxy struct {
	IP        string   `json:"ip"`
	Port      string   `json:"port"`
	Protocols []string `json:"protocols"`
}

type proxyscrapeResponse struct {
	Data []ProxyScrapeProxy `json:"proxies"`
}

type ProxyScrapeProxy struct {
	Protocol string `json:"protocol"`
	Proxy    string `json:"proxy"`
}

var (
	mu sync.RWMutex

	Socks5Valid []string
	Socks5      []string

	Socks4Valid []string
	Socks4      []string

	HttpAndSValid []string
	HttpAndS      []string

	proxyCount          = flag.Int("n", 5, "number of proxies")
	ProxyRequestTimeout = flag.Int("to", 5, "request time out")

	log            = logrus.New()
	verbosityLevel int
)

func initLogger(verbosity int) {
	switch verbosity {
	case 4:
		log.SetLevel(logrus.TraceLevel)
	case 3:
		log.SetLevel(logrus.DebugLevel)
	case 2:
		log.SetLevel(logrus.WarnLevel)
	case 1:
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.ErrorLevel)
	}
	log.SetFormatter(&logrus.TextFormatter{ForceColors: true})
}

func checkProxy(protocol string, proxy string, proxySlice *[]string, sem chan struct{}, wg *sync.WaitGroup, bar *progressbar.ProgressBar, index string) {
	defer wg.Done()

	sem <- struct{}{}
	defer func() { <-sem }()

	bar.Describe(index)

	if verbosityLevel > 0 {
		log.Debugf("Checking proxy %s with protocol %s", proxy, protocol)
	}

	ctx, cncl := context.WithTimeout(context.Background(), time.Duration(*ProxyRequestTimeout)*time.Second)
	defer cncl()

	proxy = strings.TrimSuffix(protocol+proxy, "\r")
	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("%s Error parsing proxy URL: %s", index, err.Error())
		}
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://google.com", nil)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("%s Error creating request: %s", index, err.Error())
		}
		return
	}

	req.Header.Add("User-Agent", uarand.GetRandom())
	

	resp, err := client.Do(req)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("%s Proxy %s is not working: %s", index, proxy, err.Error())
		}
		return
	}
	defer resp.Body.Close()

	if verbosityLevel > 0 {
		log.Debugf("Received response with status code %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		if verbosityLevel > 0 {
			log.Errorf("%s Proxy %s is not working: Received non-200 response: %d", index, proxy, resp.StatusCode)
		}
		return
	}

	mu.Lock()
	if len(*proxySlice) < *proxyCount {
		*proxySlice = append(*proxySlice, proxy)
		if verbosityLevel > 0 {
			log.Infof("%s Proxy %s is working!", index, proxy)
		}
		bar.Add(1)
	}
	if len(*proxySlice) == *proxyCount {
		if err := writeProxiesToFile(protocol+".txt", *proxySlice); err != nil {
			if verbosityLevel > 0 {
				log.Errorf("Error writing working proxies: %s", err)
			}
		}
		if verbosityLevel > 0 {
			log.Infof("Successfully wrote proxies to file %s.txt", protocol)
		}
		os.Exit(0)
	}
	mu.Unlock()
}

func getProxies(url string) []byte {
	if verbosityLevel > 0 {
		log.Infof("Try to retrieve proxies from URL %s", url)
	}

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("Error creating request: %s", err.Error())
		}
	}

	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("Error during request: %s", err.Error())
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("Error reading response body: %s", err.Error())
		}
		return nil
	}

	proxyCount := len(strings.Split(string(body), "\n")) - 1
	if verbosityLevel > 0 {
		log.Infof("%d proxies have been received from URL %s", proxyCount, url)
	}

	return body
}

func initProxiesSlice(urls []string, proxy *[]string) {
	var wg sync.WaitGroup
	localProxies := []string{}

	for _, url := range urls {
		wg.Add(1)

		go func(url string) {
			defer wg.Done()
			if verbosityLevel > 0 {
				log.Infof("Fetching proxies from URL %s", url)
			}
			proxies := getProxies(url)

			if proxies != nil {
				mu.Lock()
				proxiesSlice := strings.Split(string(proxies), "\n")
				localProxies = append(localProxies, proxiesSlice...)
				if verbosityLevel > 0 {
					log.Infof("Retrieved %d proxies from URL %s", len(proxiesSlice)-1, url)
				}
				mu.Unlock()
			} else {
				if verbosityLevel > 0 {
					log.Warnf("No proxies received from URL %s", url)
				}
			}
		}(url)
	}

	wg.Wait()

	mu.Lock()
	*proxy = append(*proxy, localProxies...)
	if verbosityLevel > 0 {
		log.Infof("Total proxies collected: %d", len(localProxies))
	}
	mu.Unlock()
}

func writeProxiesToFile(filename string, proxies []string) error {
	if verbosityLevel > 0 {
		log.Infof("Writing %d proxies to file %s", len(proxies), filename)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", filename, err)
	}
	defer file.Close()

	for _, proxy := range proxies {
		_, err := file.WriteString(proxy + "\n")
		if err != nil {
			return fmt.Errorf("error writing to file %s: %w", filename, err)
		}
	}

	if verbosityLevel > 0 {
		log.Infof("Successfully wrote proxies to file %s", filename)
	}

	return nil
}

func fetchGeoNodeProxies(protocol string) ([]string, error) {
	var allProxies []string
	url := fmt.Sprintf("https://proxylist.geonode.com/api/proxy-list?protocols=%s", protocol)
	page := 0

	for {
		if verbosityLevel > 0 {
			log.Infof("Fetching proxies from GeoNode, page %d", page)
		}

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

		if verbosityLevel > 0 {
			log.Infof("Retrieved %d proxies from GeoNode, page %d", len(response.Data), page)
		}
		page++
	}
	return allProxies, nil
}

func fetchProxyScrapeProxies(protocol string) ([]string, error) {
	var allProxies []string
	url := fmt.Sprintf("https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=%s&format=json", protocol)

	if verbosityLevel > 0 {
		log.Infof("Fetching proxies from ProxyScrape")
	}

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

	var proxyScrapeResponse proxyscrapeResponse
	if err := json.Unmarshal(body, &proxyScrapeResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ProxyScrape response: %v", err)
	}

	for _, proxy := range proxyScrapeResponse.Data {
		proxyString := fmt.Sprintf("%s://%s", proxy.Protocol, proxy.Proxy)
		allProxies = append(allProxies, proxyString)
	}

	if verbosityLevel > 0 {
		log.Infof("Retrieved %d proxies from ProxyScrape", len(allProxies))
	}

	return allProxies, nil
}

func fetchProxiesFromSites(protocol string) ([]string, error) {
	geonodeProxies, err := fetchGeoNodeProxies(protocol)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("Error fetching from GeoNode: %v", err)
		}
		return nil, err
	}

	proxyscrapeProxies, err := fetchProxyScrapeProxies(protocol)
	if err != nil {
		if verbosityLevel > 0 {
			log.Errorf("Error fetching from ProxyScrape: %v", err)
		}
		return nil, err
	}

	allProxies := append(geonodeProxies, proxyscrapeProxies...)
	if verbosityLevel > 0 {
		log.Infof("Fetched %d total proxies from GeoNode and ProxyScrape", len(allProxies))
	}

	return allProxies, nil
}

func removeDuplicates(slice []string) []string {
	uniqueMap := make(map[string]bool)
	uniqueSlice := []string{}

	for _, value := range slice {
		if _, exists := uniqueMap[value]; !exists {
			uniqueMap[value] = true
			uniqueSlice = append(uniqueSlice, value)
		}
	}

	if verbosityLevel > 0 {
		log.Infof("Removed duplicates, unique proxies count: %d", len(uniqueSlice))
	}

	return uniqueSlice
}

func main() {
	socks5 := flag.Bool("s5", false, "use Socks5 proxy type")
	socks4 := flag.Bool("s4", false, "use Socks4 proxy type")
	httpAndS := flag.Bool("hs", false, "use HTTP/S proxy type")
	concurrentCount := flag.Int("c", 5, "concurrency count")
	verbosity := flag.Int("v", 0, "verbosity level (1-4)")

	flag.Parse()

	verbosityLevel = *verbosity
	if verbosityLevel > 0 {
		initLogger(verbosityLevel)
	}

	if !*socks5 && !*socks4 && !*httpAndS {
		if verbosityLevel > 0 {
			log.Error("Error: Please select at least one of the flags -s5, -s4, or -hs.")
		}
		os.Exit(1)
	}

	bar := progressbar.Default(int64(*proxyCount))
	sem := make(chan struct{}, *concurrentCount)
	var wg sync.WaitGroup

	pattern := `^(socks5://|socks4://|http://|https://)`

	re := regexp.MustCompile(pattern)

	if *socks5 {
		socks5URLs := []string{
			"https://raw.githubusercontent.com/yemixzy/proxy-list/main/proxies/socks5.txt",
			"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/refs/heads/main/proxies/socks5.txt",
			"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/refs/heads/master/socks5.txt",
			"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
			"https://api.openproxylist.xyz/socks5.txt",
			"https://raw.githubusercontent.com/mmpx12/proxy-list/master/socks5.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks5.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies_anonymous/socks5.txt",
			"https://raw.githubusercontent.com/roosterkid/openproxylist/main/SOCKS5_RAW.txt",
			"https://proxyspace.pro/socks5.txt",
			"https://api.proxyscrape.com/?request=displayproxies&proxytype=socks5",
			"https://api.proxyscrape.com/v2/?request=displayproxies&protocol=socks5",
			"https://api.proxyscrape.com/v2/?request=getproxies&protocol=socks5&timeout=10000&country=all&simplified=true",
			"https://www.proxy-list.download/api/v1/get?type=socks5",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/socks5/socks5.txt",
			"https://alexa.lr2b.com/proxylist.txt",
			"https://raw.githubusercontent.com/proxifly/free-proxy-list/refs/heads/main/proxies/protocols/socks5/data.txt",
		}

		initProxiesSlice(socks5URLs, &Socks5)
		protocol := "socks5"
		proxies, err := fetchProxiesFromSites(protocol)

		if err == nil {
			Socks5 = append(Socks5, proxies...)
		}

		Socks5 = removeDuplicates(Socks5)

		for index, socks5Url := range Socks5 {
			wg.Add(1)
			order := fmt.Sprintf("%d/%d", index, len(Socks5))
			go checkProxy("socks5", "://"+re.ReplaceAllString(socks5Url, ""), &Socks5Valid, sem, &wg, bar, order)
		}
	}

	if *socks4 {
		socks4URLs := []string{
			"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/refs/heads/master/socks4.txt",
			"https://raw.githubusercontent.com/yemixzy/proxy-list/main/proxies/socks4.txt",
			"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/refs/heads/main/proxies/socks4.txt",
			"https://api.proxyscrape.com/?request=displayproxies&proxytype=socks4",
			"https://api.proxyscrape.com/v2/?request=displayproxies&protocol=socks4",
			"https://api.proxyscrape.com/v2/?request=getproxies&protocol=socks4&timeout=10000&country=all&simplified=true",
			"https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt",
			"https://proxyspace.pro/socks4.txt",
			"https://api.openproxylist.xyz/socks4.txt",
			"https://www.proxy-list.download/api/v1/get?type=socks4",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/socks4/socks4.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks4.txt",
			"https://raw.githubusercontent.com/rdavydov/proxy-list/main/proxies_anonymous/socks4.txt",
			"https://alexa.lr2b.com/proxylist.txt",
			"https://raw.githubusercontent.com/proxifly/free-proxy-list/refs/heads/main/proxies/protocols/socks4/data.txt",
		}

		initProxiesSlice(socks4URLs, &Socks4)
		protocol := "socks4"
		proxies, err := fetchProxiesFromSites(protocol)

		if err == nil {
			Socks4 = append(Socks4, proxies...)
		}

		Socks4 = removeDuplicates(Socks4)

		for index, socks4Url := range Socks4 {
			wg.Add(1)
			order := fmt.Sprintf("%d/%d", index, len(Socks4))
			go checkProxy("socks4", "://"+re.ReplaceAllString(socks4Url, ""), &Socks4Valid, sem, &wg, bar, order)
		}
	}

	if *httpAndS {
		httpURLs := []string{
			"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/refs/heads/master/https.txt",
			"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/refs/heads/master/http.txt",
			"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/refs/heads/main/proxies/http.txt",
			"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/refs/heads/main/proxies/https.txt",
			"https://raw.githubusercontent.com/yemixzy/proxy-list/main/proxies/http.txt",
			"https://proxyspace.pro/http.txt",
			"https://proxyspace.pro/https.txt",
			"https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt",
			"https://api.openproxylist.xyz/http.txt",
			"https://alexa.lr2b.com/proxylist.txt",
			"https://www.proxy-list.download/api/v1/get?type=http",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt",
			"https://www.proxy-list.download/api/v1/get?type=https",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/https/https.txt",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/http/http.txt",
			"https://raw.githubusercontent.com/proxifly/free-proxy-list/refs/heads/main/proxies/protocols/http/data.txt",
			"https://raw.githubusercontent.com/proxifly/free-proxy-list/refs/heads/main/proxies/protocols/https/data.txt",
		}

		initProxiesSlice(httpURLs, &HttpAndS)
		protocol := "http"
		proxies, err := fetchProxiesFromSites(protocol)

		if err == nil {
			HttpAndS = append(HttpAndS, proxies...)
		}

		HttpAndS = removeDuplicates(HttpAndS)

		for index, httpAndS := range HttpAndS{
			wg.Add(1)
			order := fmt.Sprintf("%d/%d", index, len(HttpAndS))
			go checkProxy("http", "://"+re.ReplaceAllString(httpAndS, ""), &HttpAndS, sem, &wg, bar, order)
		}
	}

	wg.Wait()

	if len(Socks5Valid) != 0 {
		if err := writeProxiesToFile("socks5.txt", Socks5Valid); err != nil {
			if verbosityLevel > 0 {
				log.Errorf("Error writing working proxies: %s", err)
			}
		}
		os.Exit(0)
	}

	if len(Socks4Valid) != 0 {
		if err := writeProxiesToFile("socks4.txt", Socks4Valid); err != nil {
			if verbosityLevel > 0 {
				log.Errorf("Error writing working proxies: %s", err)
			}
		}
		os.Exit(0)
	}

	if len(HttpAndSValid) != 0 {
		if err := writeProxiesToFile("http.txt", HttpAndSValid); err != nil {
			if verbosityLevel > 0 {
				log.Errorf("Error writing working proxies: %s", err)
			}
		}
		os.Exit(0)
	}

	os.Exit(0)
}