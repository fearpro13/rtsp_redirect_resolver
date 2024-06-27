package rtsp_redirect_resolver

import (
	"encoding/csv"
	"encoding/json"
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type FileExt string

const (
	Csv  FileExt = "csv"
	Json FileExt = "json"
)

type SourcesList interface {
	Add(source Source)
	Get(original string) (Source, bool)
	RefreshSources()
	ResolveSources()
	Iterate(handler func(source Source))
}

type SourceMap map[string]Source

type ConcurrentSourceMap struct {
	SourceMap
	sync.Mutex
}

func (s SourceMap) Add(source Source) {
	s[source.original] = source
}

func (s SourceMap) Get(original string) (Source, bool) {
	source, e := s[original]
	return source, e
}

func (s SourceMap) Iterate(handler func(source Source)) {
	for _, source := range s {
		handler(source)
	}
}

func (s SourceMap) ResolveSources() {
	wg := sync.WaitGroup{}

	newSources := make(SourceMap)
	wg.Add(len(s))
	for _, source := range s {
		go func() {
			newSource, err := source.UpdateFinalDestination()
			if err != nil {
				log.Println(err)
			} else {
				newSources.Add(newSource)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	for _, newSource := range newSources {
		s.Add(newSource)
	}
}

func (c *ConcurrentSourceMap) Add(source Source) {
	c.Lock()
	c.SourceMap[source.original] = source
	c.Unlock()
}

type Source struct {
	original string
	resolved string
}

func NewSource(original string) Source {
	return Source{original: original}
}

type ArgSourcesList struct {
	ConcurrentSourceMap
}

type FileSourcesList struct {
	path string
	ext  FileExt
	ConcurrentSourceMap
}

type HttpSourcesList struct {
	url string
	ConcurrentSourceMap
}

func NewHttpSourcesList(path string) *HttpSourcesList {
	return &HttpSourcesList{
		url: path,
		ConcurrentSourceMap: ConcurrentSourceMap{
			SourceMap: SourceMap{},
		},
	}
}

func (h *HttpSourcesList) RefreshSources() {
	c := http.Client{Timeout: 5 * time.Second}

	r, err := c.Get(h.url)
	if err != nil {
		log.Print(err)
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()

	httpSources, err := readSourcesFromJson(r.Body)
	if err != nil {
		log.Print(err)
		return
	}

	for _, httpSource := range httpSources {
		h.Add(httpSource)
	}
}

func NewFileSourcesList(path string, ext FileExt) *FileSourcesList {
	return &FileSourcesList{
		ext:                 ext,
		path:                path,
		ConcurrentSourceMap: ConcurrentSourceMap{SourceMap: SourceMap{}},
	}
}

func (f *FileSourcesList) RefreshSources() {
	file, err := os.Open(f.path)
	if err != nil {
		log.Println(err)
		return
	}

	defer func() {
		_ = file.Close()
	}()

	var fileSources []Source
	switch f.ext {
	case Json:
		fileSources, err = readSourcesFromJson(file)
		if err != nil {
			log.Println(err)
			return
		}
	case Csv:
		fileSources, err = readSourcesFromCsv(file)
		if err != nil {
			log.Println(err)
			return
		}
	}

	for _, fileSource := range fileSources {
		f.Add(fileSource)
	}
}

func NewArgSourcesList(sources []Source) *ArgSourcesList {
	sourceMap := make(SourceMap)

	for _, source := range sources {
		sourceMap[source.original] = source
	}

	return &ArgSourcesList{ConcurrentSourceMap{
		SourceMap: sourceMap,
	},
	}
}

func (a *ArgSourcesList) RefreshSources() {
	return
}

func readSourcesFromJson(r io.Reader) ([]Source, error) {
	var values []string
	decoder := json.NewDecoder(r)

	err := decoder.Decode(&values)

	if err != nil {
		return nil, err
	}

	sources := make([]Source, len(values))

	for _, value := range values {
		sources = append(sources, Source{
			original: value,
			resolved: "",
		})
	}

	return sources, nil
}

func readSourcesFromCsv(r io.Reader) ([]Source, error) {
	var values []string
	decoder := csv.NewReader(r)

	rows, err := decoder.ReadAll()

	if err != nil {
		return nil, err
	}

	sources := make([]Source, len(values))

	for _, row := range rows {
		sources = append(sources, Source{
			original: row[0],
			resolved: "",
		})
	}

	return sources, nil
}

func (s Source) UpdateFinalDestination() (Source, error) {
	c := gortsplib.Client{
		RedirectDisable: false,
		ReadTimeout:     15 * time.Second,
	}

	parsed, err := url.Parse(s.original)
	if err != nil {
		return Source{}, err
	}

	err = c.Start(parsed.Scheme, parsed.Host)
	if err != nil {
		return Source{}, err
	}

	defer func() {
		_ = c.Close()
	}()

	_, finalUrl, _, err := c.Describe(parsed)
	if err != nil {
		return Source{}, err
	}

	return Source{
		original: s.original,
		resolved: finalUrl.String(),
	}, nil
}
