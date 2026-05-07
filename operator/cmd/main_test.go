package main

import "testing"

func TestCacheOptions(t *testing.T) {
	t.Run("empty namespace watches all namespaces", func(t *testing.T) {
		opts := cacheOptions("")
		if len(opts.DefaultNamespaces) != 0 {
			t.Fatalf("expected no namespace restriction, got %v", opts.DefaultNamespaces)
		}
	})

	t.Run("namespace restricts cache to one namespace", func(t *testing.T) {
		opts := cacheOptions("test-garm")
		if len(opts.DefaultNamespaces) != 1 {
			t.Fatalf("expected one namespace, got %v", opts.DefaultNamespaces)
		}
		if _, ok := opts.DefaultNamespaces["test-garm"]; !ok {
			t.Fatalf("expected test-garm namespace in cache options, got %v", opts.DefaultNamespaces)
		}
	})
}
