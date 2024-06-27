package rtsp_redirect_resolver

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"os"
)

type Printer interface {
	Print(list []Source)
}

type StdOutPrinter struct {
	separator string
}

func NewStdOutPrinter(separator string) *StdOutPrinter {
	return &StdOutPrinter{separator: separator}
}

func (p *StdOutPrinter) Print(list []Source) {
	for _, source := range list {
		_, _ = os.Stdout.WriteString(source.resolved + p.separator)
	}
}

type WriterPrinter struct {
	w io.Writer
}

func NewWriterPrinter(w io.Writer) *WriterPrinter {
	return &WriterPrinter{w: w}
}

func (p *WriterPrinter) Print(list []Source) {
	encoder := json.NewEncoder(p.w)
	sources := map[string]string{}
	for _, source := range list {
		sources[source.original] = source.resolved
	}

	err := encoder.Encode(sources)
	if err != nil {
		log.Println(err)
	}
}

type CsvPrinter struct {
	w io.Writer
}

func NewCsvPrinter(w io.Writer) *CsvPrinter {
	return &CsvPrinter{w: w}
}

func (p *CsvPrinter) Print(list []Source) {
	encoder := csv.NewWriter(p.w)

	sources := [][]string{}
	for _, source := range list {
		sources = append(sources, []string{source.original, source.resolved})
	}

	err := encoder.WriteAll(sources)
	if err != nil {
		log.Println(err)
	}
}
