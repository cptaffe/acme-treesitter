package treesitter

import "testing"

func TestShebanInterpreter(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"#!/bin/sh", "sh"},
		{"#!/bin/bash", "bash"},
		{"#!/usr/bin/env bash", "bash"},
		{"#!/usr/bin/env python3", "python3"},
		{"#!/usr/bin/env python3.11", "python3.11"},
		{"#!/usr/bin/env -S scala -classpath lib", "scala"},
		{"#!/usr/bin/env -vS node", "node"},
		{"#!/usr/bin/env jbang", "jbang"},
		{"#!/usr/bin/env deno", "deno"},
		{"# not a shebang", ""},
		{"", ""},
		{"#!/usr/bin/env -S", ""}, // env -S with nothing after
	}
	for _, c := range cases {
		got := shebanInterpreter(c.line)
		if got != c.want {
			t.Errorf("shebanInterpreter(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestLangIDForInterpreter(t *testing.T) {
	cases := []struct {
		interp string
		wantID string
	}{
		{"bash", "bash"},
		{"sh", "bash"},
		{"zsh", "bash"},
		{"fish", "bash"},
		{"python", "python"},
		{"python3", "python"},
		{"python3.11", "python"},
		{"python2.7", "python"},
		{"node", "javascript"},
		{"nodejs", "javascript"},
		{"deno", "javascript"},
		{"bun", "javascript"},
		{"ts-node", "javascript"},
		{"java", "java"},
		{"jbang", "java"},
		{"scala", "scala"},
		{"scala3", "scala"},
		{"amm", "scala"},
		{"rust-script", "rust"},
		{"ruby", ""},   // not registered
		{"perl", ""},   // not registered
		{"", ""},
	}
	for _, c := range cases {
		got := langIDForInterpreter(c.interp)
		if got != c.wantID {
			t.Errorf("langIDForInterpreter(%q) = %q, want %q", c.interp, got, c.wantID)
		}
	}
}

func TestDetectByShebang(t *testing.T) {
	cases := []struct {
		line     string
		wantLang string // "" means nil expected
	}{
		{"#!/usr/bin/env python3", "python"},
		{"#!/usr/bin/env python3.11", "python"},
		{"#!/usr/bin/env bash", "bash"},
		{"#!/bin/sh", "bash"},
		{"#!/usr/bin/env scala", "scala"},
		{"#!/usr/bin/env -S scala", "scala"},
		{"#!/usr/bin/env java", "java"},
		{"#!/usr/bin/env jbang", "java"},
		{"#!/usr/bin/env node", "javascript"},
		{"#!/usr/bin/env deno", "javascript"},
		{"#!/usr/bin/env rust-script", "rust"},
		{"package main", ""},          // not a shebang
		{"#!/usr/bin/env ruby", ""},   // grammar not registered
	}
	for _, c := range cases {
		lang := detectByShebang(c.line)
		got := ""
		if lang != nil {
			got = lang.Name
		}
		if got != c.wantLang {
			t.Errorf("detectByShebang(%q) = %q, want %q", c.line, got, c.wantLang)
		}
	}
}
