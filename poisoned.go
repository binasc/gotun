package main

import (
	"os"
	"sort"
	"strings"
	"sync"
)

type PoisonedDomainImpl struct {

	trie DomainTrie

	file string

	lock sync.Mutex
}

func NewPoisonedDomain(file string) DomainTrie {
	return &PoisonedDomainImpl{
		NewDomainTrie(file),
		file,
		sync.Mutex{},
	}
}

func (pd *PoisonedDomainImpl) Add(domain string) {
	pd.trie.Add(TruncateDomain(domain))
	go pd.updatePoisoned(domain)
}

func (pd *PoisonedDomainImpl) Test(domain string) bool {
	return pd.trie.Test(domain)
}

func (pd *PoisonedDomainImpl) readPoisonedDomains() map[string]interface{} {
	domains := make(map[string]interface{})
	ReadLine(pd.file, func(line string) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			domains[line] = true
		}
	})
	return domains
}

func (pd *PoisonedDomainImpl) updatePoisoned(domain string) {
	pd.lock.Lock()
	defer pd.lock.Unlock()

	exists := pd.readPoisonedDomains()

	if _, ok := exists[domain]; ok {
		return
	}

	f, err := os.Create(pd.file)
	if err != nil {
		Error.Printf("open file error: %v\n", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			Warning.Printf("close file error: %v\n", err)
		}
	}()

	exists[domain] = true
	domains := make([]string, 0, len(exists))

	for key, _ := range exists {
		domains = append(domains, key)
	}

	sort.Strings(domains)

	for _, d := range domains {
		_, err = f.WriteString(d + "\n")
		if err != nil {
			Error.Printf("Failed to update poisoned list %v\n", err)
			break
		}
	}
}

