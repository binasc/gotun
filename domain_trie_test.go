package main

import "testing"

func TestDomainAddTest(t *testing.T) {

	trie := NewDomainTrie("")

	tests := []struct { domain string; expect bool } {
		{ "example.com", false },
		{ "example.com.", false },
	}

	for _, test := range tests {
		result := trie.Test(test.domain)
		if result != test.expect {
			t.Errorf("Expect test on %s is %v, but got %v", test.domain, test.expect, result)
		}
	}

	domains := []string {
		"google.com",
		"github.com.",
	}

	for _, domain := range domains {
		trie.Add(domain)
	}

	tests = []struct { domain string; expect bool } {
		{ "google.com", true },
		{ "google.com.", true },
		{ "www.google.com", true },
		{ "www.google.com.hk", false },
		{ "github.com", true },
		{ "github.com.", true },
		{ "www.github.com", true },
		{ "www.github.com.hk", false },
		{ ".", true },
		{ "com", false },
		{ "com.", false },
		{ "google", false },
		{ "google.", false },
	}

	for _, test := range tests {
		result := trie.Test(test.domain)
		if result != test.expect {
			t.Errorf("Expect test on %s is %v, but got %v", test.domain, test.expect, result)
		}
	}
}

func TestTruncateDomain(t *testing.T) {
	tests := []struct { domain string; expect string } {
		{ "google.com", "google.com" },
		{ "google.com.", "google.com" },
		{ "www.google.com", "google.com" },
		{ "www.google.com.hk", "google.com.hk" },
		{ "github.com", "github.com" },
		{ "github.com.", "github.com" },
		{ "www.github.com", "github.com" },
		{ "www.github.com.hk", "github.com.hk" },
		{ ".", "" },
		{ "com", "com" },
		{ "com.", "com" },
		{ "co.jp", "co.jp" },
		{ "google", "google" },
		{ "google.", "google" },
	}

	for _, test := range tests {
		result := TruncateDomain(test.domain)
		if result != test.expect {
			t.Errorf("Expect test on %s is %v, but got %v", test.domain, test.expect, result)
		}
	}
}
