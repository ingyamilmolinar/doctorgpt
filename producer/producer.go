package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Parse command-line arguments
	logFilePath := flag.String("logfile", "", "path to log file")
	delayMs := flag.Int("delay-ms", 100, "milliseconds between log lines")
	infinite := flag.String("infinite", "false", "print logs forever")
	flag.Parse()

	if *logFilePath == "" {
		log.Fatal("Log file path is required")
	}

	file, err := os.Open(*logFilePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	file.Close()

	// Read logfile path and print it eternally
	for {
		file, err := os.Open(*logFilePath)
		if err != nil {
			log.Fatalf("Error opening file: %v", err)
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
			time.Sleep(time.Duration(*delayMs) * time.Millisecond)
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
		if *infinite != "true" {
			// sleep forever
			time.Sleep(time.Duration(1<<63 - 1))
		}
	}
}
