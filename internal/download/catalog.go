package download

import "fmt"

// Entry is a downloadable GGUF file. Sizes come from the Hugging Face file
// lists recorded in docs/FINDINGS.md (6-2); the authoritative size and
// SHA256 are fetched with Lookup right before downloading.
type Entry struct {
	Params    string `json:"params"` // "0.8B" ...
	Quant     string `json:"quant"`
	Repo      string `json:"repo"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
}

var catalogSizes = []struct {
	size  string
	quant string
	bytes int64
}{
	{"0.8b", "Q4_K_M", 573126848},
	{"0.8b", "Q4_0", 547543232},
	{"0.8b", "Q3_K_M", 486385856},
	{"0.8b", "Q2_K", 410138816},
	{"2b", "Q4_K_M", 1807203040},
	{"2b", "Q4_0", 1698577120},
	{"2b", "Q3_K_M", 1495886560},
	{"2b", "Q2_K", 1225612000},
	{"8b", "Q4_K_M", 5280445728},
	{"8b", "Q3_K_M", 4310218016},
	{"8b", "Q2_K", 3450746144},
	{"31b", "Q4_K_M", 19526593952},
	{"31b", "Q3_K_M", 15863131552},
	{"31b", "Q2_K", 12609924512},
}

// Catalog returns all known PLaMo 3 Translate GGUF files.
func Catalog() []Entry {
	out := make([]Entry, 0, len(catalogSizes))
	for _, c := range catalogSizes {
		params := map[string]string{"0.8b": "0.8B", "2b": "2B", "8b": "8B", "31b": "31B"}[c.size]
		out = append(out, Entry{
			Params:    params,
			Quant:     c.quant,
			Repo:      fmt.Sprintf("pfnet/plamo-3-translate-%s-GGUF", c.size),
			File:      fmt.Sprintf("plamo-3-translate-%s-%s.gguf", c.size, c.quant),
			SizeBytes: c.bytes,
		})
	}
	return out
}

// FindEntry returns the catalog entry for file.
func FindEntry(file string) (Entry, bool) {
	for _, e := range Catalog() {
		if e.File == file {
			return e, true
		}
	}
	return Entry{}, false
}
