package digest

import (
	"encoding/csv"
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

func normalizeLines(lines [][]string, reorder Positions) [][]string {
	if len(reorder) == 0 {
		return lines
	}

	normalized := make([][]string, len(lines))
	for i, line := range lines {
		normalized[i] = normalizeLine(line, reorder)
	}

	return normalized
}

func normalizeLine(line []string, reorder Positions) []string {
	if len(reorder) == 0 {
		return line
	}

	normalized := make([]string, len(reorder))
	for i, sourcePos := range reorder {
		normalized[i] = line[sourcePos]
	}

	return normalized
}
