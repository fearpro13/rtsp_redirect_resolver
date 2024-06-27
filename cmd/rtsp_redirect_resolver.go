package main

import (
	"errors"
	"fearpro13/rtsp_redirect_resolver"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	Ok = iota
	Error
	NeedHelp
)

const (
	ReturnAsArgs     = "args"
	ReturnAsNewLines = "nl"
	ReturnAsJson     = "json"
	ReturnAsCsv      = "csv"
	ReturnHttp       = "http"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	if os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" || os.Args[1] == "?" {
		printHelp()
		os.Exit(0)
	}

	returnCode, err := run(os.Args[1:])
	if err != nil {
		log.Println(err)
	}

	if returnCode == NeedHelp {
		printHelp()
		os.Exit(1)
	}

	os.Exit(returnCode)
}

func printHelp() {
	println(`
Usage:
rtsp_redirect_resolver <format> sources...

where format is one of:
args - prints result in single row, all final sources separated by single space, each
nl - prints result in multiple rows, all final sources separated by newline \n
json - print result as json array to redirect_sources.json file
csv - print result as table to redirect_sources.csv file
http:<port>:<refresh_interval_seconds> - returns result on HTTP API at 'GET localhost:<port>/', input sources are refreshed and resolved every <refresh_interval_seconds>

supported sources:
http|https - fetches url that contains json array and adds it to input list
json - parses local file containing json array and adds it to input list
rtsp|rtsps - just adds source to input list
csv - parses local csv file and adds it to input list

example:
rtsp_redirect_resolver args rtsp://127.0.0.1/stream1 https://mybroadcast.com/broadcasts ~/local_broadcasts.json
rtsp_redirect_resolver http:8123:3600 rtsp://127.0.0.1/stream1 https://mybroadcast.com/broadcasts ~/local_broadcasts.json
`)
}

func run(args []string) (int, error) {
	if len(args) < 2 {
		return NeedHelp, errors.New("not enough arguments")
	}

	format := args[0]
	formatSupported := false
	originalFormatArg := ""
	for _, supportedFormat := range []string{
		ReturnAsArgs,
		ReturnAsNewLines,
		ReturnAsJson,
		ReturnAsCsv,
		ReturnHttp,
	} {
		if strings.HasPrefix(format, supportedFormat) {
			originalFormatArg = format
			format = supportedFormat
			formatSupported = true
			break
		}
	}

	if !formatSupported {
		return NeedHelp, errors.New("unknown format: " + format)
	}

	sourceArgs := args[1:]

	sources := make([]rtsp_redirect_resolver.SourcesList, 0)
	wg := sync.WaitGroup{}
	wg.Add(len(sourceArgs))

	m := sync.Mutex{}
	for _, sourceArg := range sourceArgs {
		go func() {
			innerSources := getSourceList(sourceArg)

			if innerSources != nil {
				m.Lock()
				sources = append(sources, innerSources)
				m.Unlock()

			}

			wg.Done()
		}()
	}

	wg.Wait()

	if len(sources) == 0 {
		return Ok, nil
	}

	if format == ReturnHttp {
		httpArgs := strings.Split(originalFormatArg, ":")
		if len(httpArgs) < 3 {
			return NeedHelp, errors.New("wrong http arguments")
		}
		portStr, intervalStr := httpArgs[1], httpArgs[2]

		interval, err := strconv.Atoi(intervalStr)
		if err != nil {
			return Error, err
		}

		refreshTicker := time.NewTicker(time.Duration(interval) * time.Second)
		go func() {
			for {
				refreshAndResolve(sources)
				<-refreshTicker.C
			}
		}()

		m := http.NewServeMux()

		s := http.Server{
			Addr:    ":" + portStr,
			Handler: m,
		}

		m.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			rtsp_redirect_resolver.NewWriterPrinter(writer).Print(extractSources(sources))
		})

		go func() {
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGKILL)

			fmt.Printf("received %v signal\n", <-sigs)
			_ = s.Shutdown(nil)
		}()

		err = s.ListenAndServe()
		if err != nil {
			log.Println(err)
		}
	} else {
		refreshAndResolve(sources)
	}

	switch format {
	case ReturnHttp:
		return Ok, nil
	case ReturnAsArgs:
		rtsp_redirect_resolver.NewStdOutPrinter(" ").Print(extractSources(sources))
		return Ok, nil
	case ReturnAsNewLines:
		rtsp_redirect_resolver.NewStdOutPrinter("\n").Print(extractSources(sources))
		return Ok, nil
	case ReturnAsJson:
		f, err := os.OpenFile("redirect_sources.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			return Error, err
		}

		defer func() {
			_ = f.Close()
		}()

		rtsp_redirect_resolver.NewWriterPrinter(f).Print(extractSources(sources))

		return Ok, nil
	case ReturnAsCsv:
		f, err := os.OpenFile("redirect_sources.csv", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			return Error, err
		}

		defer func() {
			_ = f.Close()
		}()

		rtsp_redirect_resolver.NewCsvPrinter(f).Print(extractSources(sources))

		return Ok, nil
	}

	return Error, errors.New("unknown format: " + format)
}

func refreshAndResolve(lists []rtsp_redirect_resolver.SourcesList) {
	wg := sync.WaitGroup{}

	wg.Add(len(lists))

	for _, sourceList := range lists {
		go func() {
			sourceList.RefreshSources()
			sourceList.ResolveSources()
			wg.Done()
		}()
	}

	wg.Wait()
}

func extractSources(lists []rtsp_redirect_resolver.SourcesList) []rtsp_redirect_resolver.Source {
	var sources []rtsp_redirect_resolver.Source

	for _, sourceList := range lists {
		sourceList.Iterate(func(source rtsp_redirect_resolver.Source) {
			sources = append(sources, source)
		})
	}

	return sources
}

func getSourceList(address string) rtsp_redirect_resolver.SourcesList {
	if strings.HasPrefix(address, "http") {
		return rtsp_redirect_resolver.NewHttpSourcesList(address)
	} else if strings.HasSuffix(address, ".json") {
		return rtsp_redirect_resolver.NewFileSourcesList(address, rtsp_redirect_resolver.Json)
	} else if strings.HasPrefix(address, "rtsp") {
		return rtsp_redirect_resolver.NewArgSourcesList([]rtsp_redirect_resolver.Source{
			rtsp_redirect_resolver.NewSource(address),
		})
	} else if strings.HasSuffix(address, ".csv") {
		return rtsp_redirect_resolver.NewFileSourcesList(address, rtsp_redirect_resolver.Csv)
	} else {
		log.Println("unsupported source: " + address)
	}

	return nil
}
