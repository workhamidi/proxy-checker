package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/corpix/uarand"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

var (
	mu                  sync.RWMutex
	Socks5Valid         []string
	Socks5              []string
	Socks4Valid         []string
	Socks4              []string
	HttpAndSValid       []string
	HttpAndS            []string
	proxyCount          = flag.Int("n", 5, "number of proxies")
	ProxyRequestTimeout = flag.Int("to", 5, "request time out")
	MyIp                = [3]string{
		"https://checkip.amazonaws.com",
		"https://ident.me",
		"https://ifconfig.me",
	}
)

func checkProxy(protocol string, proxy string, proxySlice *[]string, sem chan struct{}, wg *sync.WaitGroup, bar *progressbar.ProgressBar, myIpIndex int, index string) {
	defer wg.Done()

	sem <- struct{}{}
	defer func() { <-sem }()

	ctx, cncl := context.WithTimeout(context.Background(), time.Duration(*ProxyRequestTimeout)*time.Second)
	defer cncl()

	proxy = strings.TrimSuffix(protocol+proxy, "\r")
	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		color.Red("%s Error parsing proxy URL: %s\n", index, err.Error())
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", MyIp[myIpIndex], nil)
	if err != nil {
		color.Red("%s Error parsing proxy URL: %s\n", index, err.Error())
		return
	}

	req.Header.Add("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		color.Red("[-] %s Proxy %s is not working: %s\n", index, proxy, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		color.Red("[-] %s  Proxy %s is not working: Received non-200 response: %d\n", index, proxy, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		color.Red("%s Error reading response body: %s\n", index, err.Error())
		return
	}

	if string(body) == proxyUrl.Hostname() {
		mu.Lock()
		if len(*proxySlice) < *proxyCount {
			*proxySlice = append(*proxySlice, proxy)
			color.Green("[+] %s Proxy %s is working!\n", index, proxy)
			bar.Add(1)
		}
		if len(*proxySlice) == *proxyCount {
			if err := writeProxiesToFile(protocol+".txt", *proxySlice); err != nil {
				fmt.Println("Error writing working proxies:", err)
			}
			os.Exit(0)
		}
		mu.Unlock()
	} else {
		color.Red("[-] %s Proxy %s is not working: Response does not match\n", index, proxy)
	}
}

func getProxies(url string) []byte {

	color.Yellow("[/] Try to retrieve proxies from URL %s", url)

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		color.Red(err.Error(), "\n")
		os.Exit(0)
	}

	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		color.Red(err.Error(), "\n")
		os.Exit(0)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		color.Red("[-] %s", err.Error())
		return nil
	}

	color.Green("[+] Proxies have been received from URL %s", url)

	return body
}

func initProxiesSlice(urls []string, proxy *[]string) {
	var wg sync.WaitGroup
	localProxies := []string{}

	for _, url := range urls {
		wg.Add(1)

		go func(url string) {
			defer wg.Done()
			proxies := getProxies(url)

			if proxies != nil {
				mu.Lock()
				proxiesSlice := strings.Split(string(proxies), "\n")
				localProxies = append(localProxies, proxiesSlice...)
				mu.Unlock()
			}
		}(url)
	}

	wg.Wait()

	mu.Lock()
	*proxy = append(*proxy, localProxies...)
	mu.Unlock()
}

func writeProxiesToFile(filename string, proxies []string) error {
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

	return nil
}

func main() {

	socks5 := flag.Bool("s5", false, "use Socks5 proxy type")
	socks4 := flag.Bool("s4", false, "use Socks4 proxy type")
	httpAndS := flag.Bool("hs", false, "use HTTP/S proxy type")

	concurrentCount := flag.Int("c", 5, "concurrency count")

	flag.Parse()

	if !*socks5 && !*socks4 && !*httpAndS {
		color.Red("Error: Please select at least one of the flags -s5, -s4, or -hs.")
		os.Exit(1)
	}

	bar := progressbar.Default(int64(*proxyCount))

	sem := make(chan struct{}, *concurrentCount)

	var wg sync.WaitGroup

	if *socks5 {
		socks5URLs := []string{
			"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
			"https://api.openproxylist.xyz/socks5.txt",
			"https://raw.githubusercontent.com/B4RC0DE-TM/proxy-list/main/SOCKS5.txt",
			"https://raw.githubusercontent.com/mmpx12/proxy-list/master/socks5.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks5.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies_anonymous/socks5.txt",
			"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/socks5.txt",
			"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-socks5.txt",
			"https://raw.githubusercontent.com/roosterkid/openproxylist/main/SOCKS5_RAW.txt",
			"https://raw.githubusercontent.com/hookzof/socks5_list/master/proxy.txt",
			"https://proxyspace.pro/socks5.txt",
			"https://api.proxyscrape.com/?request=displayproxies&proxytype=socks5",
			"https://api.proxyscrape.com/v2/?request=displayproxies&protocol=socks5",
			"https://api.proxyscrape.com/v2/?request=getproxies&protocol=socks5&timeout=10000&country=all&simplified=true",
			"http://worm.rip/socks5.txt",
			"https://www.proxy-list.download/api/v1/get?type=socks5",
		}

		initProxiesSlice(socks5URLs, &Socks5)

		for index, socks5Url := range Socks5 {
			wg.Add(1)
			order := fmt.Sprintf("%d/%d", index, len(Socks5))
			go checkProxy("socks5", "://"+socks5Url, &Socks5Valid, sem, &wg, bar, rand.Intn(2), order)
		}
	}

	if *socks4 {
		socks4URLs := []string{
			"https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt",
			"https://api.openproxylist.xyz/socks4.txt",
			"https://www.proxy-list.download/api/v1/get?type=socks4",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/socks4/socks4.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks4.txt",
			"https://raw.githubusercontent.com/rdavydov/proxy-list/main/proxies_anonymous/socks4.txt",
		}

		initProxiesSlice(socks4URLs, &Socks4)

		for index, socks4Url := range Socks4 {
			wg.Add(1)
			order := fmt.Sprintf("%d/%d", index, len(Socks4))
			go checkProxy("socks4", "://"+socks4Url, &Socks4Valid, sem, &wg, bar, rand.Intn(2), order)
		}
	}

	if *httpAndS {
		httpURLs := []string{
			"https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt",
			"https://api.openproxylist.xyz/http.txt",
			"https://alexa.lr2b.com/proxylist.txt",
			"https://rootjazz.com/proxies/proxies.txt",
			"https://www.proxy-list.download/api/v1/get?type=http",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/http/http.txt",
			"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt",
			"https://www.proxy-list.download/api/v1/get?type=https",
			"https://raw.githubusercontent.com/officialputuid/KangProxy/KangProxy/https/https.txt",
			"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-https.txt",
		}

		initProxiesSlice(httpURLs, &HttpAndS)

		for index, httpAndS := range HttpAndS {
			wg.Add(1)
			order := fmt.Sprintf("%d/%d", index, len(HttpAndS))
			go checkProxy("http", "://"+httpAndS, &HttpAndS, sem, &wg, bar, rand.Intn(2), order)
		}
	}

	wg.Wait()

	os.Exit(0)

}
