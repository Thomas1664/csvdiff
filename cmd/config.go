package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/afero"

	"github.com/aswinkarthik/csvdiff/pkg/digest"
)

// Context is to store all command line Flags.
type Context struct {
	fs                     afero.Fs
	primaryKeyPositions    []int
	valueColumnPositions   []int
	includeColumnPositions []int
	deltaReorderPositions  []int
	format                 string
	baseFilename           string
	deltaFilename          string
	baseFile               afero.File
	deltaFile              afero.File
	recordCount            int
	separator              rune
	lazyQuotes             bool
	titles                 bool
}

// NewContext can take all CLI flags and create a cmd.Context
// Validations are done as part of this.
// File pointers are created too.
func NewContext(
	fs afero.Fs,
	primaryKeyPositions []int,
	valueColumnPositions []int,
	ignoreValueColumnPositions []int,
	includeColumnPositions []int,
	format string,
	baseFilename string,
	deltaFilename string,
	separator rune,
	lazyQuotes bool,
	titles ...bool,
) (*Context, error) {
	titlesEnabled := len(titles) > 0 && titles[0]
	baseHeaders, err := getHeaders(fs, baseFilename, separator, lazyQuotes)
	if err != nil {
		return nil, fmt.Errorf("error in base-file: %v", err)
	}

	deltaHeaders, err := getHeaders(fs, deltaFilename, separator, lazyQuotes)
	if err != nil {
		return nil, fmt.Errorf("error in delta-file: %v", err)
	}

	baseRecordCount := len(baseHeaders)
	deltaRecordCount := len(deltaHeaders)
	if baseRecordCount != deltaRecordCount {
		return nil, fmt.Errorf("base-file and delta-file columns count do not match")
	}

	if len(ignoreValueColumnPositions) > 0 && len(valueColumnPositions) > 0 {
		return nil, fmt.Errorf("only one of --columns or --ignore-columns")
	}
	if len(ignoreValueColumnPositions) > 0 {
		valueColumnPositions = inferValueColumns(baseRecordCount, ignoreValueColumnPositions)
	}

	deltaReorderPositions := digest.Positions{}
	if titlesEnabled {
		deltaReorderPositions, err = getDeltaReorderPositions(baseHeaders, deltaHeaders)
		if err != nil {
			return nil, fmt.Errorf("base-file and delta-file titles do not match: %v", err)
		}
	}

	baseFile, err := fs.Open(baseFilename)
	if err != nil {
		return nil, err
	}
	deltaFile, err := fs.Open(deltaFilename)
	if err != nil {
		return nil, err
	}
	ctx := &Context{
		fs:                     fs,
		primaryKeyPositions:    primaryKeyPositions,
		valueColumnPositions:   valueColumnPositions,
		includeColumnPositions: includeColumnPositions,
		deltaReorderPositions:  deltaReorderPositions,
		format:                 format,
		baseFilename:           baseFilename,
		deltaFilename:          deltaFilename,
		baseFile:               baseFile,
		deltaFile:              deltaFile,
		recordCount:            baseRecordCount,
		separator:              separator,
		lazyQuotes:             lazyQuotes,
		titles:                 titlesEnabled,
	}

	if err := ctx.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}

	return ctx, nil
}

// GetPrimaryKeys is to return the --primary-key flags as digest.Positions array.
func (c *Context) GetPrimaryKeys() digest.Positions {
	if len(c.primaryKeyPositions) > 0 {
		return c.primaryKeyPositions
	}
	return []int{0}
}

// GetValueColumns is to return the --columns flags as digest.Positions array.
func (c *Context) GetValueColumns() digest.Positions {
	if len(c.valueColumnPositions) > 0 {
		return c.valueColumnPositions
	}
	return []int{}
}

// GetIncludeColumnPositions is to return the --include flags as digest.Positions array.
// If empty, it is value columns
func (c Context) GetIncludeColumnPositions() digest.Positions {
	if len(c.includeColumnPositions) > 0 {
		return c.includeColumnPositions
	}
	return c.GetValueColumns()
}

// validate validates the context object
// and returns error if not valid.
func (c *Context) validate() error {
	{
		// format validation

		formatFound := false
		for _, format := range allFormats {
			if strings.ToLower(c.format) == format {
				formatFound = true
			}
		}
		if !formatFound {
			return fmt.Errorf("specified format is not valid")
		}
	}

	{
		comparator := func(element int) bool {
			return element < c.recordCount
		}

		if !assertAll(c.primaryKeyPositions, comparator) {
			return fmt.Errorf("--primary-key positions are out of bounds")
		}
		if !assertAll(c.includeColumnPositions, comparator) {
			return fmt.Errorf("--include positions are out of bounds")
		}
		if !assertAll(c.valueColumnPositions, comparator) {
			return fmt.Errorf("--columns positions are out of bounds")
		}
	}

	return nil
}

func inferValueColumns(recordCount int, ignoreValueColumns []int) digest.Positions {
	lookupMap := make(map[int]struct{})
	for _, pos := range ignoreValueColumns {
		lookupMap[pos] = struct{}{}
	}

	valueColumns := make(digest.Positions, 0)
	if len(ignoreValueColumns) > 0 {
		for i := 0; i < recordCount; i++ {
			if _, exists := lookupMap[i]; !exists {
				valueColumns = append(valueColumns, i)
			}
		}
	}

	return valueColumns
}

func assertAll(elements []int, assertFn func(element int) bool) bool {
	for _, el := range elements {
		if !assertFn(el) {
			return false
		}
	}
	return true
}

func getHeaders(fs afero.Fs, filename string, separator rune, lazyQuotes bool) ([]string, error) {
	base, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer base.Close()
	csvReader := csv.NewReader(base)
	csvReader.Comma = separator
	csvReader.LazyQuotes = lazyQuotes
	record, err := csvReader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("unable to process headers from csv file. EOF reached. invalid CSV file")
		}
		return nil, err
	}

	return record, nil
}

func getDeltaReorderPositions(baseHeaders, deltaHeaders []string) (digest.Positions, error) {
	deltaByTitle := make(map[string]int, len(deltaHeaders))
	for i, title := range deltaHeaders {
		if _, exists := deltaByTitle[title]; exists {
			return nil, fmt.Errorf("duplicate title in delta-file: %s", title)
		}
		deltaByTitle[title] = i
	}

	reorder := make(digest.Positions, len(baseHeaders))
	for i, title := range baseHeaders {
		if _, exists := deltaByTitle[title]; !exists {
			return nil, fmt.Errorf("title missing in delta-file: %s", title)
		}
		reorder[i] = deltaByTitle[title]
	}

	return reorder, nil
}

// BaseDigestConfig creates a digest.Context from cmd.Context
// that is needed to start the diff process
func (c *Context) BaseDigestConfig() (digest.Config, error) {
	return digest.Config{
		Reader:      c.baseFile,
		Value:       c.valueColumnPositions,
		Key:         c.primaryKeyPositions,
		Include:     c.includeColumnPositions,
		Separator:   c.separator,
		LazyQuotes:  c.lazyQuotes,
		SkipHeaders: c.titles,
	}, nil
}

// DeltaDigestConfig creates a digest.Context from cmd.Context
// that is needed to start the diff process
func (c *Context) DeltaDigestConfig() (digest.Config, error) {
	return digest.Config{
		Reader:      c.deltaFile,
		Value:       c.valueColumnPositions,
		Key:         c.primaryKeyPositions,
		Include:     c.includeColumnPositions,
		Separator:   c.separator,
		LazyQuotes:  c.lazyQuotes,
		Reorder:     c.deltaReorderPositions,
		SkipHeaders: c.titles,
	}, nil
}

// Close all file handles
func (c *Context) Close() {
	if c.baseFile != nil {
		_ = c.baseFile.Close()
	}
	if c.deltaFile != nil {
		_ = c.deltaFile.Close()
	}
}
