package main

import (
	"bufio"
	"log"
	"os"
)

func ReadLine(filename string, handler func(line string)) {
	f, err := os.OpenFile(filename, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Fatalf("open file error: %v\n", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			Warning.Printf("close file error: %v\n", err)
		}
	}()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		handler(sc.Text())
	}
}
