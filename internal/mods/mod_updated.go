package mods

import (
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

var updatedMu sync.Mutex

const workshopUpdatedAtRecordsFileName = "workshop_updated_at_records"

func FormatRecords(records map[string]int64) string {
	recordContent := []string{}

	for key, value := range records {
		recordContent = append(recordContent, key+" "+strconv.FormatInt(value, 10))
	}

	return strings.Join(recordContent, "\n")
}

func BuildRecords(fileContents string) map[string]int64 {
	records := map[string]int64{}

	lines := strings.Split(fileContents, "\n")

	for _, line := range lines {
		parts := strings.Split(line, " ")

		if len(parts) < 2 {
			continue
		}

		publishedID := parts[0]
		timestamp, err := strconv.ParseInt(parts[1], 10, 64)

		if err != nil || publishedID == "" {
			continue
		}

		records[publishedID] = timestamp
	}

	return records
}

func WriteRecords(dirPath string, contents string) error {
	tmpName := workshopUpdatedAtRecordsFileName + "_tmp"

	err := os.WriteFile(path.Join(dirPath, tmpName), []byte(contents), 0666)

	if err != nil {
		return err
	}

	os.Rename(path.Join(dirPath, tmpName), path.Join(dirPath, "workshop_updated_at_records.txt"))

	return nil
}

func LoadRecords(dirPath string) map[string]int64 {
	modsUpdatedAt := map[string]int64{}

	fileContents, err := os.ReadFile(path.Join(dirPath, workshopUpdatedAtRecordsFileName))

	if err != nil {
		return modsUpdatedAt
	}

	fileLines := strings.Split(string(fileContents), "\n")

	for _, line := range fileLines {
		lineParts := strings.Split(line, " ")

		publishedID := lineParts[0]
		timestamp, err := strconv.ParseInt(lineParts[1], 10, 64)

		if err != nil || publishedID == "" {
			continue
		}

		modsUpdatedAt[publishedID] = timestamp
	}

	return modsUpdatedAt
}

func AddRecord(dirPath string, publishedID string, timestamp int64) error {
	updatedMu.Lock()
	defer updatedMu.Unlock()
	recordsMap := LoadRecords(dirPath)
	recordsMap[publishedID] = timestamp
	return WriteRecords(dirPath, FormatRecords(recordsMap))
}
