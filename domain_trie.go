package main

import (
	"strings"
)

type DomainTrie interface {

	Add(domain string)

	Test(domain string) bool

}

type record []record

type DomainTrieImpl struct {

	root []record

}

func NewDomainTrie(loadFromFile string) DomainTrie {
	ret := &DomainTrieImpl{ nil }
	if loadFromFile != "" {
		readDomains(loadFromFile, ret)
	}
	return ret
}

func readDomains(filename string, trie DomainTrie) {
	ReadLine(filename, func(line string) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			trie.Add(line)
		}
	})
}

func TruncateDomain(domain string) string {
	if len(domain) == 0 {
		return domain
	} else if domain[len(domain)-1] == '.' {
		domain = domain[:len(domain)-1]
	}
	domains := strings.Split(domain, ".")
	if len(domains) < 3 {
		return domain
	}
	if len(domains[len(domains)-1]) == 2 {
		domains = domains[len(domains)-3:]
	} else {
		domains = domains[len(domains)-2:]
	}
	return strings.Join(domains, ".")
}

func appendRecord(ch rune, current *[]record) *[]record {
	if *current == nil {
		*current = make([]record, 128)
	}
	return (*[]record)(&((*current)[ch]))
}

func (d *DomainTrieImpl) Add(domain string) {
	if len(domain) == 0 {
		return
	}

	current := &d.root

	if domain[len(domain)-1] == '.' {
		domain = domain[:len(domain)-1]
	}
	domains := strings.Split(domain, ".")
	for i := len(domains) - 1; i >= 0; i-- {
		subDomain := domains[i]
		for _, ch := range subDomain {
			current = appendRecord(ch, current)
		}
		current = appendRecord('.', current)
	}
}

func (d *DomainTrieImpl) Test(domain string) bool {
	if len(domain) == 0 || d.root == nil {
		return false
	}

	if domain[len(domain)-1] == '.' {
		domain = domain[:len(domain)-1]
	}
	domain = strings.ToLower(domain)

	current := d.root
	domains := strings.Split(domain, ".")
	for i := len(domains) - 1; i >= 0; i-- {
		subDomain := domains[i]
		for pos, ch := range subDomain {
			if current == nil {
				return pos == 0
			}
			current = current[ch]
		}
		if current == nil {
			return false
		}
		current = current['.']
	}

	return current == nil
}
