package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mholt/caddy"
)

func TestConditions(t *testing.T) {
	tests := []struct {
		condition string
		isTrue    bool
		shouldErr bool
	}{
		{"a is b", false, false},
		{"a is a", true, false},
		{"a not b", true, false},
		{"a not a", false, false},
		{"a has a", true, false},
		{"a has b", false, false},
		{"ba has b", true, false},
		{"bab has b", true, false},
		{"bab has bb", false, false},
		{"a not_has a", false, false},
		{"a not_has b", true, false},
		{"ba not_has b", false, false},
		{"bab not_has b", false, false},
		{"bab not_has bb", true, false},
		{"bab starts_with bb", false, false},
		{"bab starts_with ba", true, false},
		{"bab starts_with bab", true, false},
		{"bab not_starts_with bb", true, false},
		{"bab not_starts_with ba", false, false},
		{"bab not_starts_with bab", false, false},
		{"bab ends_with bb", false, false},
		{"bab ends_with bab", true, false},
		{"bab ends_with ab", true, false},
		{"bab not_ends_with bb", true, false},
		{"bab not_ends_with ab", false, false},
		{"bab not_ends_with bab", false, false},
		{"a match *", false, true},
		{"a match a", true, false},
		{"a match .*", true, false},
		{"a match a.*", true, false},
		{"a match b.*", false, false},
		{"ba match b.*", true, false},
		{"ba match b[a-z]", true, false},
		{"b0 match b[a-z]", false, false},
		{"b0a match b[a-z]", false, false},
		{"b0a match b[a-z]+", false, false},
		{"b0a match b[a-z0-9]+", true, false},
		{"bac match b[a-z]{2}", true, false},
		{"a not_match *", false, true},
		{"a not_match a", false, false},
		{"a not_match .*", false, false},
		{"a not_match a.*", false, false},
		{"a not_match b.*", true, false},
		{"ba not_match b.*", false, false},
		{"ba not_match b[a-z]", false, false},
		{"b0 not_match b[a-z]", true, false},
		{"b0a not_match b[a-z]", true, false},
		{"b0a not_match b[a-z]+", true, false},
		{"b0a not_match b[a-z0-9]+", false, false},
		{"bac not_match b[a-z]{2}", false, false},
	}

	for i, test := range tests {
		str := strings.Fields(test.condition)
		ifCond, err := newIfCond(str[0], str[1], str[2])
		if err != nil {
			if !test.shouldErr {
				t.Error(err)
			}
			continue
		}
		isTrue := ifCond.True(nil)
		if isTrue != test.isTrue {
			t.Errorf("Test %d: '%s' expected %v found %v", i, test.condition, test.isTrue, isTrue)
		}
	}

	invalidOperators := []string{"ss", "and", "if"}
	for _, op := range invalidOperators {
		_, err := newIfCond("a", op, "b")
		if err == nil {
			t.Errorf("Invalid operator %v used, expected error.", op)
		}
	}

	replaceTests := []struct {
		url       string
		condition string
		isTrue    bool
	}{
		{"/home", "{uri} match /home", true},
		{"/hom", "{uri} match /home", false},
		{"/hom", "{uri} starts_with /home", false},
		{"/hom", "{uri} starts_with /h", true},
		{"/home/.hiddenfile", `{uri} match \/\.(.*)`, true},
		{"/home/.hiddendir/afile", `{uri} match \/\.(.*)`, true},
	}

	for i, test := range replaceTests {
		r, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			t.Errorf("Test %d: failed to create request: %v", i, err)
			continue
		}
		ctx := context.WithValue(r.Context(), OriginalURLCtxKey, *r.URL)
		r = r.WithContext(ctx)
		str := strings.Fields(test.condition)
		ifCond, err := newIfCond(str[0], str[1], str[2])
		if err != nil {
			t.Errorf("Test %d: failed to create 'if' condition %v", i, err)
			continue
		}
		isTrue := ifCond.True(r)
		if isTrue != test.isTrue {
			t.Errorf("Test %v: expected %v found %v", i, test.isTrue, isTrue)
			continue
		}
	}
}

func TestIfMatcher(t *testing.T) {
	tests := []struct {
		conditions []string
		isOr       bool
		isTrue     bool
	}{
		{
			[]string{
				"a is a",
				"b is b",
				"c is c",
			},
			false,
			true,
		},
		{
			[]string{
				"a is b",
				"b is c",
				"c is c",
			},
			true,
			true,
		},
		{
			[]string{
				"a is a",
				"b is a",
				"c is c",
			},
			false,
			false,
		},
		{
			[]string{
				"a is b",
				"b is c",
				"c is a",
			},
			true,
			false,
		},
		{
			[]string{},
			false,
			true,
		},
		{
			[]string{},
			true,
			false,
		},
	}

	for i, test := range tests {
		matcher := IfMatcher{isOr: test.isOr}
		for _, condition := range test.conditions {
			str := strings.Fields(condition)
			ifCond, err := newIfCond(str[0], str[1], str[2])
			if err != nil {
				t.Error(err)
			}
			matcher.ifs = append(matcher.ifs, ifCond)
		}
		isTrue := matcher.Match(nil)
		if isTrue != test.isTrue {
			t.Errorf("Test %d: expected %v found %v", i, test.isTrue, isTrue)
		}
	}
}

func TestSetupIfMatcher(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  IfMatcher
	}{
		{`test {
			if	a match b
		 }`, false, IfMatcher{
			ifs: []ifCond{
				{a: "a", op: "match", b: "b", neg: false},
			},
		}},
		{`test {
			if a match b
			if_op or
		 }`, false, IfMatcher{
			ifs: []ifCond{
				{a: "a", op: "match", b: "b", neg: false},
			},
			isOr: true,
		}},
		{`test {
			if	a match
		 }`, true, IfMatcher{},
		},
		{`test {
			if	a isn't b
		 }`, true, IfMatcher{},
		},
		{`test {
			if a match b c
		 }`, true, IfMatcher{},
		},
		{`test {
			if goal has go
			if cook not_has go
		 }`, false, IfMatcher{
			ifs: []ifCond{
				{a: "goal", op: "has", b: "go", neg: false},
				{a: "cook", op: "has", b: "go", neg: true},
			},
		}},
		{`test {
			if goal has go
			if cook not_has go
			if_op and
		 }`, false, IfMatcher{
			ifs: []ifCond{
				{a: "goal", op: "has", b: "go", neg: false},
				{a: "cook", op: "has", b: "go", neg: true},
			},
		}},
		{`test {
			if goal has go
			if cook not_has go
			if_op not
		 }`, true, IfMatcher{},
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("http", test.input)
		c.Next()
		matcher, err := SetupIfMatcher(c)
		if err == nil && test.shouldErr {
			t.Errorf("Test %d didn't error, but it should have", i)
		} else if err != nil && !test.shouldErr {
			t.Errorf("Test %d errored, but it shouldn't have; got '%v'", i, err)
		} else if err != nil && test.shouldErr {
			continue
		}
		if _, ok := matcher.(IfMatcher); !ok {
			t.Error("RequestMatcher should be of type IfMatcher")
		}
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
		if fmt.Sprint(matcher) != fmt.Sprint(test.expected) {
			t.Errorf("Test %v: Expected %v, found %v", i,
				fmt.Sprint(test.expected), fmt.Sprint(matcher))
		}
	}
}

func TestIfMatcherKeyword(t *testing.T) {
	tests := []struct {
		keyword  string
		expected bool
	}{
		{"if", true},
		{"ifs", false},
		{"tls", false},
		{"http", false},
		{"if_op", true},
		{"if_type", false},
		{"if_cond", false},
	}

	for i, test := range tests {
		c := caddy.NewTestController("http", test.keyword)
		c.Next()
		valid := IfMatcherKeyword(c)
		if valid != test.expected {
			t.Errorf("Test %d: expected %v found %v", i, test.expected, valid)
		}
	}
}
