package digest

import (
	"encoding/csv"
	"fmt"
	"io"
)

func getNextNLines(reader *csv.Reader) ([][]string, bool, error) {
	lines := make([][]string, bufferSize)

	lineCount := 0
	eofReached := false
	for ; lineCount < bufferSize; lineCount++ {
		line, err := reader.Read()
		lines[lineCount] = line
		if err != nil {
			if err == io.EOF {
				eofReached = true
				break
			}

			return nil, true, err
		}
	}

	return lines[:lineCount], eofReached, nil
}

func normalizeLines(lines [][]string, reorder Positions) ([][]string, error) {
	if len(reorder) == 0 {
		return lines, nil
	}

	normalized := make([][]string, len(lines))
	for i, line := range lines {
		var err error
		normalized[i], err = normalizeLine(line, reorder)
		if err != nil {
			return nil, err
		}
	}

	return normalized, nil
}

func normalizeLine(line []string, reorder Positions) ([]string, error) {
	if len(reorder) == 0 {
		return line, nil
	}

	normalized := make([]string, len(reorder))
	for i, sourcePos := range reorder {
		if sourcePos < 0 || sourcePos >= len(line) {
			return nil, fmt.Errorf("reorder position %d is out of bounds for line with %d fields", sourcePos, len(line))
		}
		normalized[i] = line[sourcePos]
	}

	return normalized, nil
}
