package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

var (
	ProxyRequestTimeout = flag.Int("to", 5, "request time out")

	log            = logrus.New()
	verbosityLevel int

	proxySources = map[int]map[string]interface{}{
		0: {
			"type": "socks5",
			"urls": []string{
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
			},
		},
		1: {
			"type": "socks54",
			"urls": []string{
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
			},
		},
		2: {"type": "http_s",
			"urls": []string{
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
			},
		},
	}
)

func main() {
	ProxyType := flag.Int("t", 1, "proxy type: \n\t (3) > http/s \n\t (2) > socks4 \n\t (1) > socks5")
	concurrentCount := flag.Int("c", 50, "concurrency count")
	verbosity := flag.Int("v", 0, "verbosity level (1-4)")
	silent := flag.Bool("s", false, "silent")
	fileName := flag.String("f", "", "Enter the file name to save")

	flag.Parse()

	verbosityLevel = *verbosity
	initLogger()

	pattern := `(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{1,5})`
	re := regexp.MustCompile(pattern)

	proxyList := InitProxiesSlice(proxySources[*ProxyType-1]["urls"].([]string))

	proxies, err := FetchProxiesFromSites(proxySources[*ProxyType-1]["type"].(string))
	if err == nil {
		proxyList = append(proxyList, proxies...)
	} else {
		logErrorf("Failed to invoke data from site err: %s", err)
	}

	proxyList = removeDuplicates(proxyList)

	var bar *progressbar.ProgressBar
	if *silent == false {
		bar = progressbar.Default(int64(len(proxyList)))
	}

	channel := make(chan string, len(proxyList))

	var wg sync.WaitGroup

	pool, err := ants.NewPoolWithFunc(*concurrentCount, func(parameters interface{}) {
		defer wg.Done()
		checkProxy(parameters, channel, bar, *silent)
	})
	if err != nil {
		logErrorf("Failed to invoke data err: %s", err)
	}

	for index, proxy := range proxyList {

		ipAndPortMatches := re.FindStringSubmatch(proxy)

		if len(ipAndPortMatches) > 0 {

			wg.Add(1)

			proxyUrl := &url.URL{
				Scheme: proxySources[*ProxyType-1]["type"].(string),
				Host:   ipAndPortMatches[0],
			}

			parameters := ProxyTask{
				ProxyUrl: proxyUrl,
				index:    fmt.Sprintf("%d/%d", index+1, len(proxyList)),
			}

			err := pool.Invoke(parameters)
			if err != nil {
				logErrorf("Failed to invoke data err: %s", err)
				defer wg.Done()
			}
		} else {
			if *silent == false {
				bar.Add(1)
			}
		}
	}

	wg.Wait()
	pool.Release()
	close(channel)

	if *fileName != "" {
		var validProxy []string

		for proxy := range channel {
			validProxy = append(validProxy, proxy)
		}

		if err := writeProxiesToFile(*fileName+".txt", validProxy); err != nil {
			logErrorf("Error writing working proxies: %s", err)
		}

	} else {
		for proxy := range channel {
			fmt.Println(proxy)
		}
	}

	os.Exit(0)
}
