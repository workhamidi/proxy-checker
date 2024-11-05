package main

import (
	"os"
)

func removeDuplicates(slice []string) []string {
	uniqueMap := make(map[string]bool)
	uniqueSlice := []string{}

	for _, value := range slice {
		if _, exists := uniqueMap[value]; !exists {
			uniqueMap[value] = true
			uniqueSlice = append(uniqueSlice, value)
		}
	}

	logInfof("Removed duplicates, unique proxies count: %d", len(uniqueSlice))

	return uniqueSlice
}

func writeProxiesToFile(filename string, proxies []string) error {

	logInfof("Writing %d proxies to file %s", len(proxies), filename)

	file, err := os.Create(filename)
	if err != nil {
		logErrorf("error creating file %s: %v", filename, err)
		return err
	}
	defer file.Close()

	for _, proxy := range proxies {
		_, err := file.WriteString(proxy + "\n")
		if err != nil {
			logErrorf("error writing to file %s: %v", filename, err)
			return err
		}
	}

	logInfof("Successfully wrote proxies to file %s", filename)

	return nil
}
